#ifndef WHISPER_WRAPPER_H
#define WHISPER_WRAPPER_H

#ifdef __cplusplus
extern "C" {
#endif

// Forward declaration
struct whisper_context;
typedef struct whisper_context whisper_context_t;

// Wrapper functions
whisper_context_t* ww_init(const char *model_path);
char* ww_full(whisper_context_t* ctx, const float *pcm, int n_samples, const char *language);
void ww_free(whisper_context_t* ctx);
void ww_free_string(char *s);

#ifdef __cplusplus
}
#endif

#endif // WHISPER_WRAPPER_H