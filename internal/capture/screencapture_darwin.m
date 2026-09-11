#import "screencapture.h"
#import <Foundation/Foundation.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <CoreGraphics/CoreGraphics.h>
#import <VideoToolbox/VideoToolbox.h>
#import <CoreMedia/CoreMedia.h>
#import <ImageIO/ImageIO.h>

@interface CaptureDelegate : NSObject <SCStreamDelegate, SCStreamOutput>
@property (nonatomic, strong) dispatch_queue_t queue;
@property (nonatomic, assign) uint8_t *latestFrame;
@property (nonatomic, assign) int latestFrameLength;
@property (nonatomic, strong) NSLock *lock;
@end

@implementation CaptureDelegate

- (instancetype)init {
    self = [super init];
    if (self) {
        _queue = dispatch_queue_create("com.lput.capture", DISPATCH_QUEUE_SERIAL);
        _lock = [[NSLock alloc] init];
    }
    return self;
}

- (void)stream:(SCStream *)stream didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer ofType:(SCStreamOutputType)type {
    if (type != SCStreamOutputTypeScreen) {
        return;
    }
    
    CVPixelBufferRef pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer);
    if (!pixelBuffer) {
        return;
    }
    
    // Create CGImage from CVPixelBuffer
    CGImageRef cgImage;
    OSStatus status = VTCreateCGImageFromCVPixelBuffer(pixelBuffer, NULL, &cgImage);
    if (status != noErr || !cgImage) {
        return;
    }
    
    // Convert to JPEG
    NSMutableData *jpegData = [NSMutableData data];
    CGImageDestinationRef dest = CGImageDestinationCreateWithData((__bridge CFMutableDataRef)jpegData, (CFStringRef)@"public.jpeg", 1, NULL);
    if (dest) {
        // Set JPEG quality
        NSDictionary *options = @{
            (__bridge NSString *)kCGImageDestinationLossyCompressionQuality: @0.5
        };
        CGImageDestinationAddImage(dest, cgImage, (__bridge CFDictionaryRef)options);
        CGImageDestinationFinalize(dest);
        CFRelease(dest);
        
        [self.lock lock];
        if (self.latestFrame != NULL) {
            free(self.latestFrame);
            self.latestFrame = NULL;
        }
        
        self.latestFrameLength = (int)jpegData.length;
        self.latestFrame = (uint8_t *)malloc(self.latestFrameLength);
        memcpy(self.latestFrame, jpegData.bytes, self.latestFrameLength);
        [self.lock unlock];
    }
    
    CGImageRelease(cgImage);
}

- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
    NSLog(@"[SCK] Stream stopped with error: %@", error);
}

@end

static SCStream *gStream = nil;
static CaptureDelegate *gDelegate = nil;

int SCK_IsAvailable(void) {
    if (@available(macOS 12.3, *)) {
        return 1;
    }
    return 0;
}

int SCK_StartCapture(int displayIndex, int maxW, int maxH, int fps) {
    if (@available(macOS 12.3, *)) {
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block int success = 0;
        
        [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
            if (error || !content) {
                dispatch_semaphore_signal(sem);
                return;
            }
            
            if (displayIndex < 0 || displayIndex >= content.displays.count) {
                dispatch_semaphore_signal(sem);
                return;
            }
            
            SCDisplay *display = content.displays[displayIndex];
            SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display excludingApplications:@[] exceptingWindows:@[]];
            
            SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];
            
            int targetW = display.width;
            int targetH = display.height;
            if (maxW > 0 && targetW > maxW) {
                targetH = targetH * maxW / targetW;
                targetW = maxW;
            }
            
            config.width = targetW;
            config.height = targetH;
            config.minimumFrameInterval = CMTimeMake(1, (int32_t)fps);
            config.queueDepth = 3;
            config.showsCursor = YES;
            
            gStream = [[SCStream alloc] initWithFilter:filter configuration:config delegate:nil];
            gDelegate = [[CaptureDelegate alloc] init];
            
            NSError *addError = nil;
            BOOL added = [gStream addStreamOutput:gDelegate type:SCStreamOutputTypeScreen sampleHandlerQueue:gDelegate.queue error:&addError];
            
            if (added) {
                [gStream startCaptureWithCompletionHandler:^(NSError *startError) {
                    if (!startError) {
                        success = 1;
                    }
                    dispatch_semaphore_signal(sem);
                }];
            } else {
                dispatch_semaphore_signal(sem);
            }
        }];
        
        dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
        return success;
    }
    return 0;
}

void SCK_StopCapture(void) {
    if (@available(macOS 12.3, *)) {
        if (gStream) {
            [gStream stopCaptureWithCompletionHandler:nil];
            gStream = nil;
        }
        if (gDelegate) {
            [gDelegate.lock lock];
            if (gDelegate.latestFrame) {
                free(gDelegate.latestFrame);
                gDelegate.latestFrame = NULL;
            }
            [gDelegate.lock unlock];
            gDelegate = nil;
        }
    }
}

uint8_t* SCK_GetLatestFrame(int* outLen) {
    if (!gDelegate) {
        if (outLen) *outLen = 0;
        return NULL;
    }
    
    uint8_t *result = NULL;
    [gDelegate.lock lock];
    if (gDelegate.latestFrame && gDelegate.latestFrameLength > 0) {
        *outLen = gDelegate.latestFrameLength;
        result = (uint8_t *)malloc(*outLen);
        memcpy(result, gDelegate.latestFrame, *outLen);
        
        // Consume the frame
        free(gDelegate.latestFrame);
        gDelegate.latestFrame = NULL;
        gDelegate.latestFrameLength = 0;
    } else {
        if (outLen) *outLen = 0;
    }
    [gDelegate.lock unlock];
    
    return result;
}
