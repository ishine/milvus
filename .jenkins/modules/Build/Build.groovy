timeout(time: 20, unit: 'MINUTES') {

    sh '. ./scripts/before-install.sh && ./scripts/check_cache.sh -l $CCACHE_ARTFACTORY_URL --cache_dir=\$CCACHE_DIR -f ccache-\$OS_NAME-\$BUILD_ENV_IMAGE_ID.tar.gz || echo \"Ccache artfactory files not found!\"'
    sh '. ./scripts/before-install.sh && ./scripts/check_cache.sh -l $GO_CACHE_ARTFACTORY_URL --cache_dir=\$(go env GOCACHE) -f go-cache-\$OS_NAME-\$BUILD_ENV_IMAGE_ID.tar.gz || echo \"Go cache artfactory files not found!\"'
    sh '. ./scripts/before-install.sh && ./scripts/check_cache.sh -l $THIRDPARTY_ARTFACTORY_URL --cache_dir=$CUSTOM_THIRDPARTY_PATH -f thirdparty-download.tar.gz || echo \"Thirdparty artfactory files not found!\"'
    sh '. ./scripts/before-install.sh && go clean --modcache && ./scripts/check_cache.sh -l $GO_MOD_ARTFACTORY_URL --cache_dir=\$GOPATH/pkg/mod -f milvus-distributed-go-mod-\$(md5sum go.mod).tar.gz || echo \"Go mod artfactory files not found!\"'

    // Zero the cache statistics (but not the configuration options)
    sh 'ccache -z'
    sh '. ./scripts/before-install.sh && make install'
    sh 'echo -e "===\n=== ccache statistics after build\n===" && ccache --show-stats'


    withCredentials([usernamePassword(credentialsId: "${env.JFROG_CREDENTIALS_ID}", usernameVariable: 'USERNAME', passwordVariable: 'PASSWORD')]) {
        sh '. ./scripts/before-install.sh && ./scripts/update_cache.sh -l $CCACHE_ARTFACTORY_URL --cache_dir=\$CCACHE_DIR -f ccache-\$OS_NAME-\$BUILD_ENV_IMAGE_ID.tar.gz -u ${USERNAME} -p ${PASSWORD}'
        sh '. ./scripts/before-install.sh && ./scripts/update_cache.sh -l $GO_CACHE_ARTFACTORY_URL --cache_dir=\$(go env GOCACHE) -f go-cache-\$OS_NAME-\$BUILD_ENV_IMAGE_ID.tar.gz -u ${USERNAME} -p ${PASSWORD}'
        sh '. ./scripts/before-install.sh && ./scripts/update_cache.sh -l $THIRDPARTY_ARTFACTORY_URL --cache_dir=$CUSTOM_THIRDPARTY_PATH -f thirdparty-download.tar.gz -u ${USERNAME} -p ${PASSWORD}'
        sh '. ./scripts/before-install.sh && ./scripts/update_cache.sh -l $GO_MOD_ARTFACTORY_URL --cache_dir=\$GOPATH/pkg/mod -f milvus-distributed-go-mod-\$(md5sum go.mod).tar.gz -u ${USERNAME} -p ${PASSWORD}'
    }
}
