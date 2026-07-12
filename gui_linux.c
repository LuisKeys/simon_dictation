#include <gtk/gtk.h>

#include "gui_linux.h"
#include "_cgo_export.h" // declares goOnMuteClicked() / goOnLangClicked() / goOnExitClicked()

static GtkWidget *gMuteButton = NULL;
static GtkWidget *gLangButton = NULL;
static int gExiting = 0; // guards against re-entrant shutdown (destroy + Exit)

static void on_mute_clicked(GtkButton *button, gpointer user_data) {
    (void)user_data;
    // goOnMuteClicked toggles dictation and returns the new enabled state.
    int enabled = goOnMuteClicked();
    gtk_button_set_label(button, enabled ? "Mute" : "Muted");
}

static void on_lang_clicked(GtkButton *button, gpointer user_data) {
    (void)user_data;
    // goOnLangClicked toggles the language and returns 1 (English) / 0 (Spanish).
    int english = goOnLangClicked();
    gtk_button_set_label(button, english ? "EN" : "ES");
}

static void on_exit_clicked(GtkButton *button, gpointer user_data) {
    (void)button;
    (void)user_data;
    if (gExiting) return;
    gExiting = 1;
    goOnExitClicked(); // graceful shutdown; never returns (os.Exit in Go)
}

// Window close ("X") shuts down cleanly too, mirroring the Exit button.
static void on_window_destroy(GtkWidget *widget, gpointer user_data) {
    (void)widget;
    (void)user_data;
    if (gExiting) return;
    gExiting = 1;
    goOnExitClicked(); // never returns
}

void gui_run(int langIsEnglish) {
    gtk_init(NULL, NULL);

    GtkWidget *window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    gtk_window_set_title(GTK_WINDOW(window), "Simon");
    gtk_window_set_keep_above(GTK_WINDOW(window), TRUE); // ~ NSFloatingWindowLevel
    gtk_window_set_resizable(GTK_WINDOW(window), FALSE);
    gtk_window_set_decorated(GTK_WINDOW(window), TRUE);
    gtk_container_set_border_width(GTK_CONTAINER(window), 8);

    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
    gtk_container_add(GTK_CONTAINER(window), box);

    gMuteButton = gtk_button_new_with_label("Mute");
    g_signal_connect(gMuteButton, "clicked", G_CALLBACK(on_mute_clicked), NULL);
    gtk_box_pack_start(GTK_BOX(box), gMuteButton, TRUE, TRUE, 0);

    gLangButton = gtk_button_new_with_label(langIsEnglish ? "EN" : "ES");
    g_signal_connect(gLangButton, "clicked", G_CALLBACK(on_lang_clicked), NULL);
    gtk_box_pack_start(GTK_BOX(box), gLangButton, TRUE, TRUE, 0);

    GtkWidget *exitButton = gtk_button_new_with_label("Exit");
    g_signal_connect(exitButton, "clicked", G_CALLBACK(on_exit_clicked), NULL);
    gtk_box_pack_start(GTK_BOX(box), exitButton, TRUE, TRUE, 0);

    g_signal_connect(window, "destroy", G_CALLBACK(on_window_destroy), NULL);

    // Place near the top-right of the primary monitor (~ the Cocoa placement).
    gtk_widget_show_all(window);
    GdkDisplay *display = gdk_display_get_default();
    if (display) {
        GdkMonitor *monitor = gdk_display_get_primary_monitor(display);
        if (!monitor)
            monitor = gdk_display_get_monitor(display, 0);
        if (monitor) {
            GdkRectangle geo;
            gdk_monitor_get_workarea(monitor, &geo);
            int ww = 0, wh = 0;
            gtk_window_get_size(GTK_WINDOW(window), &ww, &wh);
            int margin = 16;
            gtk_window_move(GTK_WINDOW(window),
                            geo.x + geo.width - ww - margin,
                            geo.y + margin);
        }
    }

    gtk_main(); // blocks the main thread until os.Exit
}

static gboolean apply_mute_label(gpointer data) {
    int enabled = GPOINTER_TO_INT(data);
    if (gMuteButton)
        gtk_button_set_label(GTK_BUTTON(gMuteButton), enabled ? "Mute" : "Muted");
    return G_SOURCE_REMOVE;
}

void gui_set_mute_label(int enabled) {
    // Called from Go; marshal onto the GTK main loop.
    g_idle_add(apply_mute_label, GINT_TO_POINTER(enabled));
}
