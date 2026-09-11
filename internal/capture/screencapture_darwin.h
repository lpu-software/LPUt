#ifndef screencapture_h
#define screencapture_h

#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

// Returns 1 if ScreenCaptureKit is available (macOS 12.3+)
int SCK_IsAvailable(void);

// Starts capturing the display at the given index.
// Returns 1 on success, 0 on failure.
int SCK_StartCapture(int displayIndex, int maxW, int maxH, int fps);

// Stops the capture.
void SCK_StopCapture(void);

// Retrieves the latest captured JPEG frame.
// Returns a pointer to the malloc'd buffer containing the JPEG, or NULL if no new frame.
// The caller is responsible for freeing the returned buffer using free().
// outLen receives the length of the buffer.
uint8_t* SCK_GetLatestFrame(int* outLen);

#ifdef __cplusplus
}
#endif

#endif /* screencapture_h */
