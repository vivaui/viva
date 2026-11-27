#import <Cocoa/Cocoa.h>
#include "../watermark_darwin.m"
#include "../cursor_owner_darwin.h"

// Exercise only the native view tree: no window, GPU, tile rasterization, or screenshot.
@interface WatermarkProbe : VivaWatermarkView
@end
@implementation WatermarkProbe
- (void)updatePattern {}
@end
@interface WatermarkTestHost : NSView
@end
@implementation WatermarkTestHost
- (BOOL)isFlipped {return YES;}
@end
int main(void) {
 @autoreleasepool {
  // Allocate only: no text rasterization, window, GPU, or screenshot.
  // The previous extended-linear/8-bit combination returned NULL on macOS.
  for (size_t scale=1; scale<=2; scale++) {
   CGContextRef context=watermarkCreateTileContext(240*scale,180*scale);
   if(context==NULL) return 8;
   if(CGBitmapContextGetBitsPerComponent(context)!=8 || CGBitmapContextGetWidth(context)!=240*scale || CGBitmapContextGetHeight(context)!=180*scale) return 9;
   CGContextRelease(context);
  }
  // Exercise real CoreText metrics and real CGPattern ownership without drawing
  // text, allocating a window, or reading pixels/screenshots.
  CGColorSpaceRef space=CGColorSpaceCreateWithName(kCGColorSpaceLinearSRGB);
  CGFloat channels[]={0.5,0.5,0.5,0.12};
  CGColorRef color=CGColorCreate(space,channels);
  for (NSString *text in @[@"DRAFT",@"测试用户 10086"]) {
   CTLineRef line=watermarkCreateLine(text,@"",12,-0.4,color);
   CGFloat ascent=0,descent=0;
   double advance=CTLineGetTypographicBounds(line,&ascent,&descent,NULL);
   CGPoint origin=watermarkTextOrigin(line,240,180);
   if(advance<=0 || fabs(origin.x+advance/2-120)>0.001 || fabs(origin.y+(ascent-descent)/2-90)>0.001) return 7;
   CFRelease(line);
  }
  CGColorRelease(color);CGColorSpaceRelease(space);
  for (size_t scale=1;scale<=2;scale++) {
   CGAffineTransform matrix=watermarkPatternMatrix(scale);
   CGSize right=CGSizeApplyAffineTransform(CGSizeMake(scale,0),matrix);
   CGSize up=CGSizeApplyAffineTransform(CGSizeMake(0,scale),matrix);
   if(right.width!=1 || right.height!=0 || up.width!=0 || up.height!=1) return 10;
   CGContextRef context=watermarkCreateTileContext(240*scale,180*scale);
   CGImageRef image=CGBitmapContextCreateImage(context);CGContextRelease(context);
   CGPatternRef pattern=watermarkCreatePattern(image,scale);
   if(pattern==NULL) {CGImageRelease(image);return 11;}
   CGPatternRelease(pattern);
  }
  for (int angle=-90;angle<=90;angle+=15) {
   CGAffineTransform rotation=CGAffineTransformMakeRotation(angle*M_PI/180);
   CGRect bounds=watermarkPatternBounds(3840,2160,rotation);
   CGRect required=CGRectApplyAffineTransform(CGRectMake(0,0,3840,2160),CGAffineTransformInvert(rotation));
   if(!CGRectContainsRect(bounds,required)) return 12;
   // Coverage uses the inverse-rotated rectangle, not a huge square plane.
   if(bounds.size.width>hypot(3840,2160)+2 || bounds.size.height>hypot(3840,2160)+2) return 13;
  }
  NSView *host=[[WatermarkTestHost alloc] initWithFrame:NSMakeRect(0,0,800,600)];
  NSView *page=[[NSView alloc] initWithFrame:host.bounds];
  WatermarkProbe *overlay=[[WatermarkProbe alloc] initWithFrame:host.bounds];
  [overlay configure:(viva_watermark){.text="测试用户 10086",.alpha=.1,.font_size=12,.tile_width=240,.tile_height=180,.angle=-25,.top=42,.left=120}];
  [host addSubview:overlay];
  [host addSubview:page positioned:NSWindowAbove relativeTo:nil];
  viva_raise_watermark(host);
  if(host.subviews.lastObject!=overlay) return 1;
  if([overlay hitTest:NSMakePoint(200,200)]!=nil || overlay.acceptsFirstResponder) return 2;
  if([host hitTest:NSMakePoint(200,200)]!=page) return 3;
  [overlay layout];
  CALayer *clip=overlay.layer.sublayers.firstObject;
  if(!clip.masksToBounds || clip.frame.origin.x!=120 || clip.frame.origin.y!=42 || clip.frame.size.width!=680 || clip.frame.size.height!=558) return 4;
  [overlay setFrameSize:NSMakeSize(1000,700)];[overlay layout];
  if(clip.frame.size.width!=880 || clip.frame.size.height!=658) return 5;
  viva_watermark_apply(host,(viva_watermark){0});
  if([host.subviews containsObject:overlay]) return 6;
  [overlay release];[page release];[host release];
 }
 return 0;
}
