// Generated by the protocol buffer compiler.  DO NOT EDIT!
// source: segcore.proto

#ifndef GOOGLE_PROTOBUF_INCLUDED_segcore_2eproto
#define GOOGLE_PROTOBUF_INCLUDED_segcore_2eproto

#include <limits>
#include <string>

#include <google/protobuf/port_def.inc>
#if PROTOBUF_VERSION < 3009000
#error This file was generated by a newer version of protoc which is
#error incompatible with your Protocol Buffer headers. Please update
#error your headers.
#endif
#if 3009000 < PROTOBUF_MIN_PROTOC_VERSION
#error This file was generated by an older version of protoc which is
#error incompatible with your Protocol Buffer headers. Please
#error regenerate this file with a newer version of protoc.
#endif

#include <google/protobuf/port_undef.inc>
#include <google/protobuf/io/coded_stream.h>
#include <google/protobuf/arena.h>
#include <google/protobuf/arenastring.h>
#include <google/protobuf/generated_message_table_driven.h>
#include <google/protobuf/generated_message_util.h>
#include <google/protobuf/inlined_string_field.h>
#include <google/protobuf/metadata.h>
#include <google/protobuf/generated_message_reflection.h>
#include <google/protobuf/message.h>
#include <google/protobuf/repeated_field.h>  // IWYU pragma: export
#include <google/protobuf/extension_set.h>  // IWYU pragma: export
#include <google/protobuf/unknown_field_set.h>
#include "schema.pb.h"
// @@protoc_insertion_point(includes)
#include <google/protobuf/port_def.inc>
#define PROTOBUF_INTERNAL_EXPORT_segcore_2eproto
PROTOBUF_NAMESPACE_OPEN
namespace internal {
class AnyMetadata;
}  // namespace internal
PROTOBUF_NAMESPACE_CLOSE

// Internal implementation detail -- do not use these members.
struct TableStruct_segcore_2eproto {
  static const ::PROTOBUF_NAMESPACE_ID::internal::ParseTableField entries[]
    PROTOBUF_SECTION_VARIABLE(protodesc_cold);
  static const ::PROTOBUF_NAMESPACE_ID::internal::AuxillaryParseTableField aux[]
    PROTOBUF_SECTION_VARIABLE(protodesc_cold);
  static const ::PROTOBUF_NAMESPACE_ID::internal::ParseTable schema[3]
    PROTOBUF_SECTION_VARIABLE(protodesc_cold);
  static const ::PROTOBUF_NAMESPACE_ID::internal::FieldMetadata field_metadata[];
  static const ::PROTOBUF_NAMESPACE_ID::internal::SerializationTable serialization_table[];
  static const ::PROTOBUF_NAMESPACE_ID::uint32 offsets[];
};
extern const ::PROTOBUF_NAMESPACE_ID::internal::DescriptorTable descriptor_table_segcore_2eproto;
namespace milvus {
namespace proto {
namespace segcore {
class LoadFieldMeta;
class LoadFieldMetaDefaultTypeInternal;
extern LoadFieldMetaDefaultTypeInternal _LoadFieldMeta_default_instance_;
class LoadSegmentMeta;
class LoadSegmentMetaDefaultTypeInternal;
extern LoadSegmentMetaDefaultTypeInternal _LoadSegmentMeta_default_instance_;
class RetrieveResults;
class RetrieveResultsDefaultTypeInternal;
extern RetrieveResultsDefaultTypeInternal _RetrieveResults_default_instance_;
}  // namespace segcore
}  // namespace proto
}  // namespace milvus
PROTOBUF_NAMESPACE_OPEN
template<> ::milvus::proto::segcore::LoadFieldMeta* Arena::CreateMaybeMessage<::milvus::proto::segcore::LoadFieldMeta>(Arena*);
template<> ::milvus::proto::segcore::LoadSegmentMeta* Arena::CreateMaybeMessage<::milvus::proto::segcore::LoadSegmentMeta>(Arena*);
template<> ::milvus::proto::segcore::RetrieveResults* Arena::CreateMaybeMessage<::milvus::proto::segcore::RetrieveResults>(Arena*);
PROTOBUF_NAMESPACE_CLOSE
namespace milvus {
namespace proto {
namespace segcore {

// ===================================================================

class RetrieveResults :
    public ::PROTOBUF_NAMESPACE_ID::Message /* @@protoc_insertion_point(class_definition:milvus.proto.segcore.RetrieveResults) */ {
 public:
  RetrieveResults();
  virtual ~RetrieveResults();

  RetrieveResults(const RetrieveResults& from);
  RetrieveResults(RetrieveResults&& from) noexcept
    : RetrieveResults() {
    *this = ::std::move(from);
  }

  inline RetrieveResults& operator=(const RetrieveResults& from) {
    CopyFrom(from);
    return *this;
  }
  inline RetrieveResults& operator=(RetrieveResults&& from) noexcept {
    if (GetArenaNoVirtual() == from.GetArenaNoVirtual()) {
      if (this != &from) InternalSwap(&from);
    } else {
      CopyFrom(from);
    }
    return *this;
  }

  static const ::PROTOBUF_NAMESPACE_ID::Descriptor* descriptor() {
    return GetDescriptor();
  }
  static const ::PROTOBUF_NAMESPACE_ID::Descriptor* GetDescriptor() {
    return GetMetadataStatic().descriptor;
  }
  static const ::PROTOBUF_NAMESPACE_ID::Reflection* GetReflection() {
    return GetMetadataStatic().reflection;
  }
  static const RetrieveResults& default_instance();

  static void InitAsDefaultInstance();  // FOR INTERNAL USE ONLY
  static inline const RetrieveResults* internal_default_instance() {
    return reinterpret_cast<const RetrieveResults*>(
               &_RetrieveResults_default_instance_);
  }
  static constexpr int kIndexInFileMessages =
    0;

  friend void swap(RetrieveResults& a, RetrieveResults& b) {
    a.Swap(&b);
  }
  inline void Swap(RetrieveResults* other) {
    if (other == this) return;
    InternalSwap(other);
  }

  // implements Message ----------------------------------------------

  inline RetrieveResults* New() const final {
    return CreateMaybeMessage<RetrieveResults>(nullptr);
  }

  RetrieveResults* New(::PROTOBUF_NAMESPACE_ID::Arena* arena) const final {
    return CreateMaybeMessage<RetrieveResults>(arena);
  }
  void CopyFrom(const ::PROTOBUF_NAMESPACE_ID::Message& from) final;
  void MergeFrom(const ::PROTOBUF_NAMESPACE_ID::Message& from) final;
  void CopyFrom(const RetrieveResults& from);
  void MergeFrom(const RetrieveResults& from);
  PROTOBUF_ATTRIBUTE_REINITIALIZES void Clear() final;
  bool IsInitialized() const final;

  size_t ByteSizeLong() const final;
  #if GOOGLE_PROTOBUF_ENABLE_EXPERIMENTAL_PARSER
  const char* _InternalParse(const char* ptr, ::PROTOBUF_NAMESPACE_ID::internal::ParseContext* ctx) final;
  #else
  bool MergePartialFromCodedStream(
      ::PROTOBUF_NAMESPACE_ID::io::CodedInputStream* input) final;
  #endif  // GOOGLE_PROTOBUF_ENABLE_EXPERIMENTAL_PARSER
  void SerializeWithCachedSizes(
      ::PROTOBUF_NAMESPACE_ID::io::CodedOutputStream* output) const final;
  ::PROTOBUF_NAMESPACE_ID::uint8* InternalSerializeWithCachedSizesToArray(
      ::PROTOBUF_NAMESPACE_ID::uint8* target) const final;
  int GetCachedSize() const final { return _cached_size_.Get(); }

  private:
  inline void SharedCtor();
  inline void SharedDtor();
  void SetCachedSize(int size) const final;
  void InternalSwap(RetrieveResults* other);
  friend class ::PROTOBUF_NAMESPACE_ID::internal::AnyMetadata;
  static ::PROTOBUF_NAMESPACE_ID::StringPiece FullMessageName() {
    return "milvus.proto.segcore.RetrieveResults";
  }
  private:
  inline ::PROTOBUF_NAMESPACE_ID::Arena* GetArenaNoVirtual() const {
    return nullptr;
  }
  inline void* MaybeArenaPtr() const {
    return nullptr;
  }
  public:

  ::PROTOBUF_NAMESPACE_ID::Metadata GetMetadata() const final;
  private:
  static ::PROTOBUF_NAMESPACE_ID::Metadata GetMetadataStatic() {
    ::PROTOBUF_NAMESPACE_ID::internal::AssignDescriptors(&::descriptor_table_segcore_2eproto);
    return ::descriptor_table_segcore_2eproto.file_level_metadata[kIndexInFileMessages];
  }

  public:

  // nested types ----------------------------------------------------

  // accessors -------------------------------------------------------

  enum : int {
    kOffsetFieldNumber = 2,
    kFieldsDataFieldNumber = 3,
    kIdsFieldNumber = 1,
  };
  // repeated int64 offset = 2;
  int offset_size() const;
  void clear_offset();
  ::PROTOBUF_NAMESPACE_ID::int64 offset(int index) const;
  void set_offset(int index, ::PROTOBUF_NAMESPACE_ID::int64 value);
  void add_offset(::PROTOBUF_NAMESPACE_ID::int64 value);
  const ::PROTOBUF_NAMESPACE_ID::RepeatedField< ::PROTOBUF_NAMESPACE_ID::int64 >&
      offset() const;
  ::PROTOBUF_NAMESPACE_ID::RepeatedField< ::PROTOBUF_NAMESPACE_ID::int64 >*
      mutable_offset();

  // repeated .milvus.proto.schema.FieldData fields_data = 3;
  int fields_data_size() const;
  void clear_fields_data();
  ::milvus::proto::schema::FieldData* mutable_fields_data(int index);
  ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::schema::FieldData >*
      mutable_fields_data();
  const ::milvus::proto::schema::FieldData& fields_data(int index) const;
  ::milvus::proto::schema::FieldData* add_fields_data();
  const ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::schema::FieldData >&
      fields_data() const;

  // .milvus.proto.schema.IDs ids = 1;
  bool has_ids() const;
  void clear_ids();
  const ::milvus::proto::schema::IDs& ids() const;
  ::milvus::proto::schema::IDs* release_ids();
  ::milvus::proto::schema::IDs* mutable_ids();
  void set_allocated_ids(::milvus::proto::schema::IDs* ids);

  // @@protoc_insertion_point(class_scope:milvus.proto.segcore.RetrieveResults)
 private:
  class _Internal;

  ::PROTOBUF_NAMESPACE_ID::internal::InternalMetadataWithArena _internal_metadata_;
  ::PROTOBUF_NAMESPACE_ID::RepeatedField< ::PROTOBUF_NAMESPACE_ID::int64 > offset_;
  mutable std::atomic<int> _offset_cached_byte_size_;
  ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::schema::FieldData > fields_data_;
  ::milvus::proto::schema::IDs* ids_;
  mutable ::PROTOBUF_NAMESPACE_ID::internal::CachedSize _cached_size_;
  friend struct ::TableStruct_segcore_2eproto;
};
// -------------------------------------------------------------------

class LoadFieldMeta :
    public ::PROTOBUF_NAMESPACE_ID::Message /* @@protoc_insertion_point(class_definition:milvus.proto.segcore.LoadFieldMeta) */ {
 public:
  LoadFieldMeta();
  virtual ~LoadFieldMeta();

  LoadFieldMeta(const LoadFieldMeta& from);
  LoadFieldMeta(LoadFieldMeta&& from) noexcept
    : LoadFieldMeta() {
    *this = ::std::move(from);
  }

  inline LoadFieldMeta& operator=(const LoadFieldMeta& from) {
    CopyFrom(from);
    return *this;
  }
  inline LoadFieldMeta& operator=(LoadFieldMeta&& from) noexcept {
    if (GetArenaNoVirtual() == from.GetArenaNoVirtual()) {
      if (this != &from) InternalSwap(&from);
    } else {
      CopyFrom(from);
    }
    return *this;
  }

  static const ::PROTOBUF_NAMESPACE_ID::Descriptor* descriptor() {
    return GetDescriptor();
  }
  static const ::PROTOBUF_NAMESPACE_ID::Descriptor* GetDescriptor() {
    return GetMetadataStatic().descriptor;
  }
  static const ::PROTOBUF_NAMESPACE_ID::Reflection* GetReflection() {
    return GetMetadataStatic().reflection;
  }
  static const LoadFieldMeta& default_instance();

  static void InitAsDefaultInstance();  // FOR INTERNAL USE ONLY
  static inline const LoadFieldMeta* internal_default_instance() {
    return reinterpret_cast<const LoadFieldMeta*>(
               &_LoadFieldMeta_default_instance_);
  }
  static constexpr int kIndexInFileMessages =
    1;

  friend void swap(LoadFieldMeta& a, LoadFieldMeta& b) {
    a.Swap(&b);
  }
  inline void Swap(LoadFieldMeta* other) {
    if (other == this) return;
    InternalSwap(other);
  }

  // implements Message ----------------------------------------------

  inline LoadFieldMeta* New() const final {
    return CreateMaybeMessage<LoadFieldMeta>(nullptr);
  }

  LoadFieldMeta* New(::PROTOBUF_NAMESPACE_ID::Arena* arena) const final {
    return CreateMaybeMessage<LoadFieldMeta>(arena);
  }
  void CopyFrom(const ::PROTOBUF_NAMESPACE_ID::Message& from) final;
  void MergeFrom(const ::PROTOBUF_NAMESPACE_ID::Message& from) final;
  void CopyFrom(const LoadFieldMeta& from);
  void MergeFrom(const LoadFieldMeta& from);
  PROTOBUF_ATTRIBUTE_REINITIALIZES void Clear() final;
  bool IsInitialized() const final;

  size_t ByteSizeLong() const final;
  #if GOOGLE_PROTOBUF_ENABLE_EXPERIMENTAL_PARSER
  const char* _InternalParse(const char* ptr, ::PROTOBUF_NAMESPACE_ID::internal::ParseContext* ctx) final;
  #else
  bool MergePartialFromCodedStream(
      ::PROTOBUF_NAMESPACE_ID::io::CodedInputStream* input) final;
  #endif  // GOOGLE_PROTOBUF_ENABLE_EXPERIMENTAL_PARSER
  void SerializeWithCachedSizes(
      ::PROTOBUF_NAMESPACE_ID::io::CodedOutputStream* output) const final;
  ::PROTOBUF_NAMESPACE_ID::uint8* InternalSerializeWithCachedSizesToArray(
      ::PROTOBUF_NAMESPACE_ID::uint8* target) const final;
  int GetCachedSize() const final { return _cached_size_.Get(); }

  private:
  inline void SharedCtor();
  inline void SharedDtor();
  void SetCachedSize(int size) const final;
  void InternalSwap(LoadFieldMeta* other);
  friend class ::PROTOBUF_NAMESPACE_ID::internal::AnyMetadata;
  static ::PROTOBUF_NAMESPACE_ID::StringPiece FullMessageName() {
    return "milvus.proto.segcore.LoadFieldMeta";
  }
  private:
  inline ::PROTOBUF_NAMESPACE_ID::Arena* GetArenaNoVirtual() const {
    return nullptr;
  }
  inline void* MaybeArenaPtr() const {
    return nullptr;
  }
  public:

  ::PROTOBUF_NAMESPACE_ID::Metadata GetMetadata() const final;
  private:
  static ::PROTOBUF_NAMESPACE_ID::Metadata GetMetadataStatic() {
    ::PROTOBUF_NAMESPACE_ID::internal::AssignDescriptors(&::descriptor_table_segcore_2eproto);
    return ::descriptor_table_segcore_2eproto.file_level_metadata[kIndexInFileMessages];
  }

  public:

  // nested types ----------------------------------------------------

  // accessors -------------------------------------------------------

  enum : int {
    kMinTimestampFieldNumber = 1,
    kMaxTimestampFieldNumber = 2,
    kRowCountFieldNumber = 3,
  };
  // int64 min_timestamp = 1;
  void clear_min_timestamp();
  ::PROTOBUF_NAMESPACE_ID::int64 min_timestamp() const;
  void set_min_timestamp(::PROTOBUF_NAMESPACE_ID::int64 value);

  // int64 max_timestamp = 2;
  void clear_max_timestamp();
  ::PROTOBUF_NAMESPACE_ID::int64 max_timestamp() const;
  void set_max_timestamp(::PROTOBUF_NAMESPACE_ID::int64 value);

  // int64 row_count = 3;
  void clear_row_count();
  ::PROTOBUF_NAMESPACE_ID::int64 row_count() const;
  void set_row_count(::PROTOBUF_NAMESPACE_ID::int64 value);

  // @@protoc_insertion_point(class_scope:milvus.proto.segcore.LoadFieldMeta)
 private:
  class _Internal;

  ::PROTOBUF_NAMESPACE_ID::internal::InternalMetadataWithArena _internal_metadata_;
  ::PROTOBUF_NAMESPACE_ID::int64 min_timestamp_;
  ::PROTOBUF_NAMESPACE_ID::int64 max_timestamp_;
  ::PROTOBUF_NAMESPACE_ID::int64 row_count_;
  mutable ::PROTOBUF_NAMESPACE_ID::internal::CachedSize _cached_size_;
  friend struct ::TableStruct_segcore_2eproto;
};
// -------------------------------------------------------------------

class LoadSegmentMeta :
    public ::PROTOBUF_NAMESPACE_ID::Message /* @@protoc_insertion_point(class_definition:milvus.proto.segcore.LoadSegmentMeta) */ {
 public:
  LoadSegmentMeta();
  virtual ~LoadSegmentMeta();

  LoadSegmentMeta(const LoadSegmentMeta& from);
  LoadSegmentMeta(LoadSegmentMeta&& from) noexcept
    : LoadSegmentMeta() {
    *this = ::std::move(from);
  }

  inline LoadSegmentMeta& operator=(const LoadSegmentMeta& from) {
    CopyFrom(from);
    return *this;
  }
  inline LoadSegmentMeta& operator=(LoadSegmentMeta&& from) noexcept {
    if (GetArenaNoVirtual() == from.GetArenaNoVirtual()) {
      if (this != &from) InternalSwap(&from);
    } else {
      CopyFrom(from);
    }
    return *this;
  }

  static const ::PROTOBUF_NAMESPACE_ID::Descriptor* descriptor() {
    return GetDescriptor();
  }
  static const ::PROTOBUF_NAMESPACE_ID::Descriptor* GetDescriptor() {
    return GetMetadataStatic().descriptor;
  }
  static const ::PROTOBUF_NAMESPACE_ID::Reflection* GetReflection() {
    return GetMetadataStatic().reflection;
  }
  static const LoadSegmentMeta& default_instance();

  static void InitAsDefaultInstance();  // FOR INTERNAL USE ONLY
  static inline const LoadSegmentMeta* internal_default_instance() {
    return reinterpret_cast<const LoadSegmentMeta*>(
               &_LoadSegmentMeta_default_instance_);
  }
  static constexpr int kIndexInFileMessages =
    2;

  friend void swap(LoadSegmentMeta& a, LoadSegmentMeta& b) {
    a.Swap(&b);
  }
  inline void Swap(LoadSegmentMeta* other) {
    if (other == this) return;
    InternalSwap(other);
  }

  // implements Message ----------------------------------------------

  inline LoadSegmentMeta* New() const final {
    return CreateMaybeMessage<LoadSegmentMeta>(nullptr);
  }

  LoadSegmentMeta* New(::PROTOBUF_NAMESPACE_ID::Arena* arena) const final {
    return CreateMaybeMessage<LoadSegmentMeta>(arena);
  }
  void CopyFrom(const ::PROTOBUF_NAMESPACE_ID::Message& from) final;
  void MergeFrom(const ::PROTOBUF_NAMESPACE_ID::Message& from) final;
  void CopyFrom(const LoadSegmentMeta& from);
  void MergeFrom(const LoadSegmentMeta& from);
  PROTOBUF_ATTRIBUTE_REINITIALIZES void Clear() final;
  bool IsInitialized() const final;

  size_t ByteSizeLong() const final;
  #if GOOGLE_PROTOBUF_ENABLE_EXPERIMENTAL_PARSER
  const char* _InternalParse(const char* ptr, ::PROTOBUF_NAMESPACE_ID::internal::ParseContext* ctx) final;
  #else
  bool MergePartialFromCodedStream(
      ::PROTOBUF_NAMESPACE_ID::io::CodedInputStream* input) final;
  #endif  // GOOGLE_PROTOBUF_ENABLE_EXPERIMENTAL_PARSER
  void SerializeWithCachedSizes(
      ::PROTOBUF_NAMESPACE_ID::io::CodedOutputStream* output) const final;
  ::PROTOBUF_NAMESPACE_ID::uint8* InternalSerializeWithCachedSizesToArray(
      ::PROTOBUF_NAMESPACE_ID::uint8* target) const final;
  int GetCachedSize() const final { return _cached_size_.Get(); }

  private:
  inline void SharedCtor();
  inline void SharedDtor();
  void SetCachedSize(int size) const final;
  void InternalSwap(LoadSegmentMeta* other);
  friend class ::PROTOBUF_NAMESPACE_ID::internal::AnyMetadata;
  static ::PROTOBUF_NAMESPACE_ID::StringPiece FullMessageName() {
    return "milvus.proto.segcore.LoadSegmentMeta";
  }
  private:
  inline ::PROTOBUF_NAMESPACE_ID::Arena* GetArenaNoVirtual() const {
    return nullptr;
  }
  inline void* MaybeArenaPtr() const {
    return nullptr;
  }
  public:

  ::PROTOBUF_NAMESPACE_ID::Metadata GetMetadata() const final;
  private:
  static ::PROTOBUF_NAMESPACE_ID::Metadata GetMetadataStatic() {
    ::PROTOBUF_NAMESPACE_ID::internal::AssignDescriptors(&::descriptor_table_segcore_2eproto);
    return ::descriptor_table_segcore_2eproto.file_level_metadata[kIndexInFileMessages];
  }

  public:

  // nested types ----------------------------------------------------

  // accessors -------------------------------------------------------

  enum : int {
    kMetasFieldNumber = 1,
    kTotalSizeFieldNumber = 2,
  };
  // repeated .milvus.proto.segcore.LoadFieldMeta metas = 1;
  int metas_size() const;
  void clear_metas();
  ::milvus::proto::segcore::LoadFieldMeta* mutable_metas(int index);
  ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::segcore::LoadFieldMeta >*
      mutable_metas();
  const ::milvus::proto::segcore::LoadFieldMeta& metas(int index) const;
  ::milvus::proto::segcore::LoadFieldMeta* add_metas();
  const ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::segcore::LoadFieldMeta >&
      metas() const;

  // int64 total_size = 2;
  void clear_total_size();
  ::PROTOBUF_NAMESPACE_ID::int64 total_size() const;
  void set_total_size(::PROTOBUF_NAMESPACE_ID::int64 value);

  // @@protoc_insertion_point(class_scope:milvus.proto.segcore.LoadSegmentMeta)
 private:
  class _Internal;

  ::PROTOBUF_NAMESPACE_ID::internal::InternalMetadataWithArena _internal_metadata_;
  ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::segcore::LoadFieldMeta > metas_;
  ::PROTOBUF_NAMESPACE_ID::int64 total_size_;
  mutable ::PROTOBUF_NAMESPACE_ID::internal::CachedSize _cached_size_;
  friend struct ::TableStruct_segcore_2eproto;
};
// ===================================================================


// ===================================================================

#ifdef __GNUC__
  #pragma GCC diagnostic push
  #pragma GCC diagnostic ignored "-Wstrict-aliasing"
#endif  // __GNUC__
// RetrieveResults

// .milvus.proto.schema.IDs ids = 1;
inline bool RetrieveResults::has_ids() const {
  return this != internal_default_instance() && ids_ != nullptr;
}
inline const ::milvus::proto::schema::IDs& RetrieveResults::ids() const {
  const ::milvus::proto::schema::IDs* p = ids_;
  // @@protoc_insertion_point(field_get:milvus.proto.segcore.RetrieveResults.ids)
  return p != nullptr ? *p : *reinterpret_cast<const ::milvus::proto::schema::IDs*>(
      &::milvus::proto::schema::_IDs_default_instance_);
}
inline ::milvus::proto::schema::IDs* RetrieveResults::release_ids() {
  // @@protoc_insertion_point(field_release:milvus.proto.segcore.RetrieveResults.ids)
  
  ::milvus::proto::schema::IDs* temp = ids_;
  ids_ = nullptr;
  return temp;
}
inline ::milvus::proto::schema::IDs* RetrieveResults::mutable_ids() {
  
  if (ids_ == nullptr) {
    auto* p = CreateMaybeMessage<::milvus::proto::schema::IDs>(GetArenaNoVirtual());
    ids_ = p;
  }
  // @@protoc_insertion_point(field_mutable:milvus.proto.segcore.RetrieveResults.ids)
  return ids_;
}
inline void RetrieveResults::set_allocated_ids(::milvus::proto::schema::IDs* ids) {
  ::PROTOBUF_NAMESPACE_ID::Arena* message_arena = GetArenaNoVirtual();
  if (message_arena == nullptr) {
    delete reinterpret_cast< ::PROTOBUF_NAMESPACE_ID::MessageLite*>(ids_);
  }
  if (ids) {
    ::PROTOBUF_NAMESPACE_ID::Arena* submessage_arena = nullptr;
    if (message_arena != submessage_arena) {
      ids = ::PROTOBUF_NAMESPACE_ID::internal::GetOwnedMessage(
          message_arena, ids, submessage_arena);
    }
    
  } else {
    
  }
  ids_ = ids;
  // @@protoc_insertion_point(field_set_allocated:milvus.proto.segcore.RetrieveResults.ids)
}

// repeated int64 offset = 2;
inline int RetrieveResults::offset_size() const {
  return offset_.size();
}
inline void RetrieveResults::clear_offset() {
  offset_.Clear();
}
inline ::PROTOBUF_NAMESPACE_ID::int64 RetrieveResults::offset(int index) const {
  // @@protoc_insertion_point(field_get:milvus.proto.segcore.RetrieveResults.offset)
  return offset_.Get(index);
}
inline void RetrieveResults::set_offset(int index, ::PROTOBUF_NAMESPACE_ID::int64 value) {
  offset_.Set(index, value);
  // @@protoc_insertion_point(field_set:milvus.proto.segcore.RetrieveResults.offset)
}
inline void RetrieveResults::add_offset(::PROTOBUF_NAMESPACE_ID::int64 value) {
  offset_.Add(value);
  // @@protoc_insertion_point(field_add:milvus.proto.segcore.RetrieveResults.offset)
}
inline const ::PROTOBUF_NAMESPACE_ID::RepeatedField< ::PROTOBUF_NAMESPACE_ID::int64 >&
RetrieveResults::offset() const {
  // @@protoc_insertion_point(field_list:milvus.proto.segcore.RetrieveResults.offset)
  return offset_;
}
inline ::PROTOBUF_NAMESPACE_ID::RepeatedField< ::PROTOBUF_NAMESPACE_ID::int64 >*
RetrieveResults::mutable_offset() {
  // @@protoc_insertion_point(field_mutable_list:milvus.proto.segcore.RetrieveResults.offset)
  return &offset_;
}

// repeated .milvus.proto.schema.FieldData fields_data = 3;
inline int RetrieveResults::fields_data_size() const {
  return fields_data_.size();
}
inline ::milvus::proto::schema::FieldData* RetrieveResults::mutable_fields_data(int index) {
  // @@protoc_insertion_point(field_mutable:milvus.proto.segcore.RetrieveResults.fields_data)
  return fields_data_.Mutable(index);
}
inline ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::schema::FieldData >*
RetrieveResults::mutable_fields_data() {
  // @@protoc_insertion_point(field_mutable_list:milvus.proto.segcore.RetrieveResults.fields_data)
  return &fields_data_;
}
inline const ::milvus::proto::schema::FieldData& RetrieveResults::fields_data(int index) const {
  // @@protoc_insertion_point(field_get:milvus.proto.segcore.RetrieveResults.fields_data)
  return fields_data_.Get(index);
}
inline ::milvus::proto::schema::FieldData* RetrieveResults::add_fields_data() {
  // @@protoc_insertion_point(field_add:milvus.proto.segcore.RetrieveResults.fields_data)
  return fields_data_.Add();
}
inline const ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::schema::FieldData >&
RetrieveResults::fields_data() const {
  // @@protoc_insertion_point(field_list:milvus.proto.segcore.RetrieveResults.fields_data)
  return fields_data_;
}

// -------------------------------------------------------------------

// LoadFieldMeta

// int64 min_timestamp = 1;
inline void LoadFieldMeta::clear_min_timestamp() {
  min_timestamp_ = PROTOBUF_LONGLONG(0);
}
inline ::PROTOBUF_NAMESPACE_ID::int64 LoadFieldMeta::min_timestamp() const {
  // @@protoc_insertion_point(field_get:milvus.proto.segcore.LoadFieldMeta.min_timestamp)
  return min_timestamp_;
}
inline void LoadFieldMeta::set_min_timestamp(::PROTOBUF_NAMESPACE_ID::int64 value) {
  
  min_timestamp_ = value;
  // @@protoc_insertion_point(field_set:milvus.proto.segcore.LoadFieldMeta.min_timestamp)
}

// int64 max_timestamp = 2;
inline void LoadFieldMeta::clear_max_timestamp() {
  max_timestamp_ = PROTOBUF_LONGLONG(0);
}
inline ::PROTOBUF_NAMESPACE_ID::int64 LoadFieldMeta::max_timestamp() const {
  // @@protoc_insertion_point(field_get:milvus.proto.segcore.LoadFieldMeta.max_timestamp)
  return max_timestamp_;
}
inline void LoadFieldMeta::set_max_timestamp(::PROTOBUF_NAMESPACE_ID::int64 value) {
  
  max_timestamp_ = value;
  // @@protoc_insertion_point(field_set:milvus.proto.segcore.LoadFieldMeta.max_timestamp)
}

// int64 row_count = 3;
inline void LoadFieldMeta::clear_row_count() {
  row_count_ = PROTOBUF_LONGLONG(0);
}
inline ::PROTOBUF_NAMESPACE_ID::int64 LoadFieldMeta::row_count() const {
  // @@protoc_insertion_point(field_get:milvus.proto.segcore.LoadFieldMeta.row_count)
  return row_count_;
}
inline void LoadFieldMeta::set_row_count(::PROTOBUF_NAMESPACE_ID::int64 value) {
  
  row_count_ = value;
  // @@protoc_insertion_point(field_set:milvus.proto.segcore.LoadFieldMeta.row_count)
}

// -------------------------------------------------------------------

// LoadSegmentMeta

// repeated .milvus.proto.segcore.LoadFieldMeta metas = 1;
inline int LoadSegmentMeta::metas_size() const {
  return metas_.size();
}
inline void LoadSegmentMeta::clear_metas() {
  metas_.Clear();
}
inline ::milvus::proto::segcore::LoadFieldMeta* LoadSegmentMeta::mutable_metas(int index) {
  // @@protoc_insertion_point(field_mutable:milvus.proto.segcore.LoadSegmentMeta.metas)
  return metas_.Mutable(index);
}
inline ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::segcore::LoadFieldMeta >*
LoadSegmentMeta::mutable_metas() {
  // @@protoc_insertion_point(field_mutable_list:milvus.proto.segcore.LoadSegmentMeta.metas)
  return &metas_;
}
inline const ::milvus::proto::segcore::LoadFieldMeta& LoadSegmentMeta::metas(int index) const {
  // @@protoc_insertion_point(field_get:milvus.proto.segcore.LoadSegmentMeta.metas)
  return metas_.Get(index);
}
inline ::milvus::proto::segcore::LoadFieldMeta* LoadSegmentMeta::add_metas() {
  // @@protoc_insertion_point(field_add:milvus.proto.segcore.LoadSegmentMeta.metas)
  return metas_.Add();
}
inline const ::PROTOBUF_NAMESPACE_ID::RepeatedPtrField< ::milvus::proto::segcore::LoadFieldMeta >&
LoadSegmentMeta::metas() const {
  // @@protoc_insertion_point(field_list:milvus.proto.segcore.LoadSegmentMeta.metas)
  return metas_;
}

// int64 total_size = 2;
inline void LoadSegmentMeta::clear_total_size() {
  total_size_ = PROTOBUF_LONGLONG(0);
}
inline ::PROTOBUF_NAMESPACE_ID::int64 LoadSegmentMeta::total_size() const {
  // @@protoc_insertion_point(field_get:milvus.proto.segcore.LoadSegmentMeta.total_size)
  return total_size_;
}
inline void LoadSegmentMeta::set_total_size(::PROTOBUF_NAMESPACE_ID::int64 value) {
  
  total_size_ = value;
  // @@protoc_insertion_point(field_set:milvus.proto.segcore.LoadSegmentMeta.total_size)
}

#ifdef __GNUC__
  #pragma GCC diagnostic pop
#endif  // __GNUC__
// -------------------------------------------------------------------

// -------------------------------------------------------------------


// @@protoc_insertion_point(namespace_scope)

}  // namespace segcore
}  // namespace proto
}  // namespace milvus

// @@protoc_insertion_point(global_scope)

#include <google/protobuf/port_undef.inc>
#endif  // GOOGLE_PROTOBUF_INCLUDED_GOOGLE_PROTOBUF_INCLUDED_segcore_2eproto