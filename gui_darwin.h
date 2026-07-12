#ifndef GUI_DARWIN_H
#define GUI_DARWIN_H

// gui_run sets up a small floating control window (near the top-right of the
// main screen) with "Mute" and "Exit" buttons, then enters the Cocoa run
// loop. It MUST be called from the main OS thread and never returns (the
// process exits via the Exit button or a termination signal).
void gui_run(void);

// gui_set_mute_label updates the mute button title from Go on the main queue.
// Reserved for a future state-sync feature; not wired in the MVP.
void gui_set_mute_label(int enabled);

#endif
