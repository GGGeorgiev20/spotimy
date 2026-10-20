package main

/*
#cgo pkg-config: webkit2gtk-4.1
#include <webkit2/webkit2.h>

static gboolean setCacheModel(gpointer data) {
	webkit_web_context_set_cache_model(webkit_web_context_get_default(), WEBKIT_CACHE_MODEL_DOCUMENT_VIEWER);
	return G_SOURCE_REMOVE;
}

static void useDocumentViewerCache(void) { g_idle_add(setCacheModel, NULL); }
*/
import "C"

// lowMemoryWebKit turns off WebKit's memory and disk caches. Images already
// come from our own disk cache, so WebKit keeping its own copies only costs RAM.
// It runs on the GTK main loop once Wails has created the webview.
func lowMemoryWebKit() { C.useDocumentViewerCache() }
