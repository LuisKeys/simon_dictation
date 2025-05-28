// whisper_wrapper.cpp
#include "whisper_wrapper.h"
#include "/home/lucho/projects/ai/whisper.cpp/build/install/include/whisper.h" // Full path to whisper.h
#include <string>
#include <cstdlib>
#include <cstring>


extern "C" {

whisper_context_t * ww_init(const char *model_path) {
    struct whisper_context_params params = whisper_context_default_params();
    return whisper_init_from_file_with_params(model_path, params);
}

char * ww_full(whisper_context_t * ctx,
               const float *pcm,
               int n_samples,
               const char *language) {
    // run full whisper inference
    struct whisper_full_params params = whisper_full_default_params(WHISPER_SAMPLING_GREEDY);
    if (language) {
        params.language = language;
    }
    // feed the raw audio
    int ret = whisper_full(ctx, params, pcm, n_samples);
    if (ret != 0) {
        return nullptr;
    }
    // grab the result text
    const char *txt = whisper_full_get_segment_text(ctx, 0);  
    // you can accumulate multiple segments if you want
    size_t len = strlen(txt);
    char *out = (char*) malloc(len + 1);
    memcpy(out, txt, len + 1);
    return out;
}

void ww_free_string(char *s) {
    free(s);
}

void ww_free(whisper_context_t *ctx) {
    whisper_free(ctx);
}

} // extern "C"
