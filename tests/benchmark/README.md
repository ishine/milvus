# Quick start

### Description：

This project is used to test performance/reliability/stability for milvus server
- Test cases can be organized with `yaml`
- Test can run with local mode or helm mode

### Usage:
`pip install requirements.txt`

if using local mode, the following libs is optional

`pymongo==3.10.0` 

`kubernetes==10.0.1`

### Demos：

1. Local test：

   `python3 main.py --local --host=*.* --port=19530 --suite=suites/gpu_search_performance_random50m.yaml`

### Definitions of test suites：

Testers need to write test suite config if adding a customizised test into the current test framework

1. search_performance: the test type，also we have`build_performance`,`insert_performance`,`accuracy`,`stability`,`search_stability`
2. tables: list of test cases
3. The following fields are in the `table` field：
   - server: run host
   - milvus: config in milvus
   - collection_name: currently support one collection
   - run_count: search count
   - search_params: params of query

## Test result：

Test result will be uploaded if tests run in helm mode, and will be used to judge if the test run pass or failed
