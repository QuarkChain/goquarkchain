%module native
%include "stdint.i"
%include "typemaps.i"

%{
#include "qkchash.h"
%}

/* Translate between go slice and c pointer . */
%typemap(gotype) (uint64_t *cache_ptr, uint32_t cache_size) %{[]uint64%}
%typemap(in) (uint64_t *cache_ptr, uint32_t cache_size) {
   $1 = ($1_ltype)$input.array;
   $2 = (uint32_t)$input.len;
}

%typemap(gotype) (uint64_t *seed_ptr) %{[]uint64%}
%typemap(in) (uint64_t *seed_ptr) {
   $1 = ($1_ltype)$input.array;
}

%typemap(gotype) (uint64_t *result_ptr) %{[]uint64%}
%typemap(in) (uint64_t *result_ptr) {
   $1 = ($1_ltype)$input.array;
}

%include "qkchash.h"
