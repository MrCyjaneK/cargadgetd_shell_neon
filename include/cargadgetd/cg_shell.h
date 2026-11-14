#ifndef CARGADGETD_CG_SHELL_H
#define CARGADGETD_CG_SHELL_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/*
 * Shell ABI. libcargadgetd_shell_<id>.so exports cargadgetd_shell_<id>_*.
 * Strings and icon bytes are valid only for the duration of the call.
 *
 *   void cargadgetd_shell_<id>_init(int frame_ms, int width, int height);
 *   void cargadgetd_shell_<id>_app(const char *id, const char *name,
 *       const uint8_t *icon, int icon_w, int icon_h, int icon_stride,
 *       cargadgetd_app_draw_fn draw, cargadgetd_app_touch_fn touch,
 *       cargadgetd_app_ctrl_fn pause, cargadgetd_app_ctrl_fn resume);
 *   void cargadgetd_shell_<id>_notif(const char *title, const char *body,
 *       const uint8_t *icon, int icon_w, int icon_h, int icon_stride);
 *   void cargadgetd_shell_<id>_draw(uint32_t *rgba, int w, int h, int stride,
 *       int64_t now_unix_ms);
 *   void cargadgetd_shell_<id>_touch(int x, int y, int action);
 *   void cargadgetd_shell_<id>_huinput(const char *event, int pressed);
 *   void cargadgetd_shell_<id>_gps_fix(int has_pos, double lat, double lon,
 *       double acc_m, int has_speed, int kmh, int has_course, double course,
 *       const char *err); // optional; latest fix, replayed on load
 *   void cargadgetd_shell_<id>_gps_sat(int gain, const char *kicker,
 *       const char *title, const char *sub, const char *detail,
 *       const char *flag); // optional; one-shot
 *   void cargadgetd_shell_<id>_stop(void);
 *
 * Optional settings, same suffixes and rules as cargadgetd_app_<id>_settings_*.
 * Missing settings_count means the shell has none. The host loader does not
 * dlsym these.
 *
 *   int         cargadgetd_shell_<id>_settings_count(void);
 *   const char *cargadgetd_shell_<id>_settings_label(int i);
 *   int         cargadgetd_shell_<id>_settings_type(int i);
 *   const char *cargadgetd_shell_<id>_settings_enum_values(int i);
 *   int         cargadgetd_shell_<id>_settings_get_bool(int i);
 *   const char *cargadgetd_shell_<id>_settings_get_enum(int i);
 *   int         cargadgetd_shell_<id>_settings_get_int(int i);
 *   int         cargadgetd_shell_<id>_settings_set_bool(int i, int v);
 *   int         cargadgetd_shell_<id>_settings_set_enum(int i, const char *v);
 *   int         cargadgetd_shell_<id>_settings_set_int(int i, int v);
 *   const char *cargadgetd_shell_<id>_settings_error(void);
 *
 * draw/touch/pause/resume run on the video-tick thread, one app at a time.
 * action: 0 up, 1 down, 2 move. Touch coordinates are in that app's buffer.
 */

typedef void (*cargadgetd_app_draw_fn)(uint32_t *rgba, int w, int h, int stride);
typedef void (*cargadgetd_app_touch_fn)(int x, int y, int action);
typedef void (*cargadgetd_app_ctrl_fn)(void);

#ifdef __cplusplus
}
#endif

#endif
