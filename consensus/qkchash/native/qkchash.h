extern "C" void *cache_create(uint64_t *cache_ptr, uint32_t cache_size);
extern "C" void cache_destroy(void *ptr);
extern "C" void qkc_hash(void *cache_ptr, uint64_t *seed_ptr, uint64_t *result_ptr);
