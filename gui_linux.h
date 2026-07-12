#ifndef GUI_LINUX_H
#define GUI_LINUX_H

// gui_run sets up a small floating control window (near the top-right of the
// primary monitor) with "Mute", language and "Exit" buttons, then enters the
// GTK main loop. langIsEnglish sets the initial language-button title (1 =>
// "EN", 0 => "ES"). It MUST be called from the main OS thread and never
// returns (the process exits via the Exit button, the window close button, or
// a termination signal).
void gui_run(int langIsEnglish);

// gui_set_mute_label updates the mute button title from Go on the GTK main
// loop. Reserved for a future state-sync feature; not wired in the MVP.
void gui_set_mute_label(int enabled);

#endif
