// Copyright (C) 2019-2020 Zilliz. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied. See the License for the specific language governing permissions and limitations under the License

#include "segcore/SegmentSealedImpl.h"
#include "query/SearchOnSealed.h"
#include "query/ScalarIndex.h"
#include "query/SearchBruteForce.h"
namespace milvus::segcore {

static inline void
set_bit(boost::dynamic_bitset<>& bitset, FieldOffset field_offset, bool flag = true) {
    bitset[field_offset.get()] = flag;
}

static inline bool
get_bit(const boost::dynamic_bitset<>& bitset, FieldOffset field_offset) {
    return bitset[field_offset.get()];
}

void
SegmentSealedImpl::LoadIndex(const LoadIndexInfo& info) {
    // NOTE: lock only when data is ready to avoid starvation
    auto field_id = FieldId(info.field_id);
    auto field_offset = schema_->get_offset(field_id);

    Assert(info.index_params.count("metric_type"));
    auto metric_type_str = info.index_params.at("metric_type");
    auto row_count = info.index->Count();
    Assert(row_count > 0);

    std::unique_lock lck(mutex_);
    Assert(!get_bit(vecindex_ready_bitset_, field_offset));
    if (row_count_opt_.has_value()) {
        AssertInfo(row_count_opt_.value() == row_count, "load data has different row count from other columns");
    } else {
        row_count_opt_ = row_count;
    }
    Assert(!vecindexs_.is_ready(field_offset));
    vecindexs_.append_field_indexing(field_offset, GetMetricType(metric_type_str), info.index);

    set_bit(vecindex_ready_bitset_, field_offset, true);
    lck.unlock();
}

void
SegmentSealedImpl::LoadFieldData(const LoadFieldDataInfo& info) {
    // NOTE: lock only when data is ready to avoid starvation
    Assert(info.row_count > 0);
    auto field_id = FieldId(info.field_id);
    Assert(info.blob);
    Assert(info.row_count > 0);
    if (SystemProperty::Instance().IsSystem(field_id)) {
        auto system_field_type = SystemProperty::Instance().GetSystemFieldType(field_id);
        Assert(system_field_type == SystemFieldType::RowId);
        auto src_ptr = reinterpret_cast<const idx_t*>(info.blob);

        // prepare data
        aligned_vector<idx_t> vec_data(info.row_count);
        std::copy_n(src_ptr, info.row_count, vec_data.data());

        // write data under lock
        std::unique_lock lck(mutex_);
        update_row_count(info.row_count);
        AssertInfo(row_ids_.empty(), "already exists");
        row_ids_ = std::move(vec_data);
        ++system_ready_count_;

    } else {
        // prepare data
        auto field_offset = schema_->get_offset(field_id);
        auto& field_meta = schema_->operator[](field_offset);
        // Assert(!field_meta.is_vector());
        auto element_sizeof = field_meta.get_sizeof();
        auto span = SpanBase(info.blob, info.row_count, element_sizeof);
        auto length_in_bytes = element_sizeof * info.row_count;
        aligned_vector<char> vec_data(length_in_bytes);
        memcpy(vec_data.data(), info.blob, length_in_bytes);

        // generate scalar index
        std::unique_ptr<knowhere::Index> index;
        if (!field_meta.is_vector()) {
            index = query::generate_scalar_index(span, field_meta.get_data_type());
        }

        // write data under lock
        std::unique_lock lck(mutex_);
        update_row_count(info.row_count);
        AssertInfo(field_datas_[field_offset.get()].empty(), "field data already exists");

        if (field_meta.is_vector()) {
            AssertInfo(!vecindexs_.is_ready(field_offset), "field data can't be loaded when indexing exists");
            field_datas_[field_offset.get()] = std::move(vec_data);
        } else {
            AssertInfo(!scalar_indexings_[field_offset.get()], "scalar indexing not cleared");
            field_datas_[field_offset.get()] = std::move(vec_data);
            scalar_indexings_[field_offset.get()] = std::move(index);
        }

        set_bit(field_data_ready_bitset_, field_offset, true);
    }
}

int64_t
SegmentSealedImpl::num_chunk_index(FieldOffset field_offset) const {
    return 1;
}

int64_t
SegmentSealedImpl::num_chunk() const {
    return 1;
}

int64_t
SegmentSealedImpl::size_per_chunk() const {
    return get_row_count();
}

SpanBase
SegmentSealedImpl::chunk_data_impl(FieldOffset field_offset, int64_t chunk_id) const {
    std::shared_lock lck(mutex_);
    Assert(get_bit(field_data_ready_bitset_, field_offset));
    auto& field_meta = schema_->operator[](field_offset);
    auto element_sizeof = field_meta.get_sizeof();
    SpanBase base(field_datas_[field_offset.get()].data(), row_count_opt_.value(), element_sizeof);
    return base;
}

const knowhere::Index*
SegmentSealedImpl::chunk_index_impl(FieldOffset field_offset, int64_t chunk_id) const {
    // TODO: support scalar index
    auto ptr = scalar_indexings_[field_offset.get()].get();
    Assert(ptr);
    return ptr;
}

int64_t
SegmentSealedImpl::GetMemoryUsageInBytes() const {
    // TODO: add estimate for index
    std::shared_lock lck(mutex_);
    auto row_count = row_count_opt_.value_or(0);
    return schema_->get_total_sizeof() * row_count;
}

int64_t
SegmentSealedImpl::get_row_count() const {
    std::shared_lock lck(mutex_);
    return row_count_opt_.value_or(0);
}

const Schema&
SegmentSealedImpl::get_schema() const {
    return *schema_;
}

void
SegmentSealedImpl::vector_search(int64_t vec_count,
                                 query::QueryInfo query_info,
                                 const void* query_data,
                                 int64_t query_count,
                                 const BitsetView& bitset,
                                 QueryResult& output) const {
    auto field_offset = query_info.field_offset_;
    auto& field_meta = schema_->operator[](field_offset);

    Assert(field_meta.is_vector());
    if (get_bit(vecindex_ready_bitset_, field_offset)) {
        Assert(vecindexs_.is_ready(field_offset));
        query::SearchOnSealed(*schema_, vecindexs_, query_info, query_data, query_count, bitset, output);
    } else if (get_bit(field_data_ready_bitset_, field_offset)) {
        query::dataset::QueryDataset dataset;
        dataset.query_data = query_data;
        dataset.num_queries = query_count;
        // if(field_meta.is)
        dataset.metric_type = query_info.metric_type_;
        dataset.topk = query_info.topK_;
        dataset.dim = field_meta.get_dim();

        Assert(get_bit(field_data_ready_bitset_, field_offset));
        Assert(row_count_opt_.has_value());
        auto row_count = row_count_opt_.value();
        auto chunk_data = field_datas_[field_offset.get()].data();

        auto sub_qr = [&] {
            if (field_meta.get_data_type() == DataType::VECTOR_FLOAT) {
                return query::FloatSearchBruteForce(dataset, chunk_data, row_count, bitset);
            } else {
                return query::BinarySearchBruteForce(dataset, chunk_data, row_count, bitset);
            }
        }();

        QueryResult results;
        results.result_distances_ = std::move(sub_qr.mutable_values());
        results.internal_seg_offsets_ = std::move(sub_qr.mutable_labels());
        results.topK_ = dataset.topk;
        results.num_queries_ = dataset.num_queries;

        output = std::move(results);
    } else {
        PanicInfo("Field Data is not loaded");
    }
}

void
SegmentSealedImpl::DropFieldData(const FieldId field_id) {
    if (SystemProperty::Instance().IsSystem(field_id)) {
        auto system_field_type = SystemProperty::Instance().GetSystemFieldType(field_id);
        Assert(system_field_type == SystemFieldType::RowId);

        std::unique_lock lck(mutex_);
        --system_ready_count_;
        auto row_ids = std::move(row_ids_);
        lck.unlock();

        row_ids.clear();
    } else {
        auto field_offset = schema_->get_offset(field_id);
        auto& field_meta = schema_->operator[](field_offset);

        std::unique_lock lck(mutex_);
        set_bit(field_data_ready_bitset_, field_offset, false);
        auto vec = std::move(field_datas_[field_offset.get()]);
        lck.unlock();

        vec.clear();
    }
}

void
SegmentSealedImpl::DropIndex(const FieldId field_id) {
    Assert(!SystemProperty::Instance().IsSystem(field_id));
    auto field_offset = schema_->get_offset(field_id);
    auto& field_meta = schema_->operator[](field_offset);
    Assert(field_meta.is_vector());

    std::unique_lock lck(mutex_);
    vecindexs_.drop_field_indexing(field_offset);
    set_bit(vecindex_ready_bitset_, field_offset, false);
}

void
SegmentSealedImpl::check_search(const query::Plan* plan) const {
    Assert(plan);
    Assert(plan->extra_info_opt_.has_value());

    if (!is_system_field_ready()) {
        PanicInfo("System Field RowID is not loaded");
    }

    auto& request_fields = plan->extra_info_opt_.value().involved_fields_;
    auto field_ready_bitset = field_data_ready_bitset_ | vecindex_ready_bitset_;
    Assert(request_fields.size() == field_ready_bitset.size());
    auto absent_fields = request_fields - field_ready_bitset;

    if (absent_fields.any()) {
        auto field_offset = FieldOffset(absent_fields.find_first());
        auto& field_meta = schema_->operator[](field_offset);
        PanicInfo("User Field(" + field_meta.get_name().get() + ") is not loaded");
    }
}

SegmentSealedImpl::SegmentSealedImpl(SchemaPtr schema)
    : schema_(schema),
      field_datas_(schema->size()),
      field_data_ready_bitset_(schema->size()),
      vecindex_ready_bitset_(schema->size()),
      scalar_indexings_(schema->size()) {
}
void
SegmentSealedImpl::bulk_subscript(SystemFieldType system_type,
                                  const int64_t* seg_offsets,
                                  int64_t count,
                                  void* output) const {
    Assert(is_system_field_ready());
    Assert(system_type == SystemFieldType::RowId);
    bulk_subscript_impl<int64_t>(row_ids_.data(), seg_offsets, count, output);
}
template <typename T>
void
SegmentSealedImpl::bulk_subscript_impl(const void* src_raw, const int64_t* seg_offsets, int64_t count, void* dst_raw) {
    static_assert(IsScalar<T>);
    auto src = reinterpret_cast<const T*>(src_raw);
    auto dst = reinterpret_cast<T*>(dst_raw);
    for (int64_t i = 0; i < count; ++i) {
        auto offset = seg_offsets[i];
        dst[i] = offset == -1 ? -1 : src[offset];
    }
}

// for vector
void
SegmentSealedImpl::bulk_subscript_impl(
    int64_t element_sizeof, const void* src_raw, const int64_t* seg_offsets, int64_t count, void* dst_raw) {
    auto src_vec = reinterpret_cast<const char*>(src_raw);
    auto dst_vec = reinterpret_cast<char*>(dst_raw);
    std::vector<char> none(element_sizeof, 0);
    for (int64_t i = 0; i < count; ++i) {
        auto offset = seg_offsets[i];
        auto dst = dst_vec + i * element_sizeof;
        const char* src;
        if (offset != 0) {
            src = src_vec + element_sizeof * offset;
        } else {
            src = none.data();
        }
        memcpy(dst, src, element_sizeof);
    }
}

void
SegmentSealedImpl::bulk_subscript(FieldOffset field_offset,
                                  const int64_t* seg_offsets,
                                  int64_t count,
                                  void* output) const {
    Assert(get_bit(field_data_ready_bitset_, field_offset));
    auto& field_meta = schema_->operator[](field_offset);
    auto src_vec = field_datas_[field_offset.get()].data();
    switch (field_meta.get_data_type()) {
        case DataType::BOOL: {
            bulk_subscript_impl<bool>(src_vec, seg_offsets, count, output);
            break;
        }
        case DataType::INT8: {
            bulk_subscript_impl<int8_t>(src_vec, seg_offsets, count, output);
            break;
        }
        case DataType::INT16: {
            bulk_subscript_impl<int16_t>(src_vec, seg_offsets, count, output);
            break;
        }
        case DataType::INT32: {
            bulk_subscript_impl<int32_t>(src_vec, seg_offsets, count, output);
            break;
        }
        case DataType::INT64: {
            bulk_subscript_impl<int64_t>(src_vec, seg_offsets, count, output);
            break;
        }
        case DataType::FLOAT: {
            bulk_subscript_impl<float>(src_vec, seg_offsets, count, output);
            break;
        }
        case DataType::DOUBLE: {
            bulk_subscript_impl<double>(src_vec, seg_offsets, count, output);
            break;
        }

        case DataType::VECTOR_FLOAT:
        case DataType::VECTOR_BINARY: {
            bulk_subscript_impl(field_meta.get_sizeof(), src_vec, seg_offsets, count, output);
            break;
        }

        default: {
            PanicInfo("unsupported");
        }
    }
}

bool
SegmentSealedImpl::HasIndex(FieldId field_id) const {
    std::shared_lock lck(mutex_);
    Assert(!SystemProperty::Instance().IsSystem(field_id));
    auto field_offset = schema_->get_offset(field_id);
    return get_bit(vecindex_ready_bitset_, field_offset);
}

bool
SegmentSealedImpl::HasFieldData(FieldId field_id) const {
    std::shared_lock lck(mutex_);
    if (SystemProperty::Instance().IsSystem(field_id)) {
        return is_system_field_ready();
    } else {
        auto field_offset = schema_->get_offset(field_id);
        return get_bit(field_data_ready_bitset_, field_offset);
    }
}

SegmentSealedPtr
CreateSealedSegment(SchemaPtr schema) {
    return std::make_unique<SegmentSealedImpl>(schema);
}

}  // namespace milvus::segcore
