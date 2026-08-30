// GoatKit Editor Partial — the single entry point for plugins that want the
// platform rich-text editor.
//
// Platform pongo2 pages include the Tiptap asset pair through
// templates/partials/tiptap_editor.pongo2 (their inline scripts call
// TiptapEditor.* synchronously, so the pair must stay eagerly loaded there).
// Plugins generate HTML from Go strings and cannot include pongo2 partials,
// so they include THIS one script instead:
//
//   <script src="/static/js/gk-editor.js"></script>
//   <script>
//     GoatKitEditor.init({ id: 'myEditor', content: '…', editorMode: 'markdown' })
//       .then(function (editor) { /* editor is the Tiptap instanceApi */ });
//   </script>
//
// That replaces the per-plugin copy-paste of the two script tags plus the
// manual "retry until TiptapEditor is defined" dance (first consumers:
// goat-kb article editor, goatcoach transcript/article editors).
//
// API (all Promise-returning calls queue on the same load promise):
//   ready()                       → Promise resolved once assets are loaded
//   isReady()                     → sync boolean
//   init(opts)                    → Promise<instanceApi>
//       opts = { id (required), content, editorMode ('richtext'|'markdown'),
//                mode ('edit'|'view'), placeholder, onUpdate(htmlOrMarkdown),
//                imageUploadUrl, imageUrlPrompt, linkUrlPrompt }
//       Re-initialising a known id updates its content instead of silently
//       returning the cached instance (underlying initTiptapEditor ignores
//       options on a cache hit).
//   content(id)                   → current content in the editor's active mode
//                                    ('' until the editor instance exists)
//   set(id, content)              → set content (fires onUpdate); calls made
//                                    before the editor instance exists are
//                                    queued and applied after init
//   setMode(id, mode)             → 'richtext' ↔ 'markdown'
//   destroy(id)                   → tear the editor down
//   insertText(id, text)          → literal insert at the cursor

(function () {
    var ASSETS = ["/static/js/tiptap.min.js", "/static/js/tiptap-editor.js"];
    var loading = null;
    var known = {}; // elementId → true once init() has been called for it
    var initialized = {}; // elementId → true once init() has completed
    var pendingSet = {}; // elementId → content queued until the editor exists

    // tiptap.min.js sets window.TiptapEditor to the raw Editor *class*;
    // tiptap-editor.js then replaces it with the wrapper object carrying
    // init/getContent/… So readiness means ".init is a function".
    function editorReady() {
        return (
            typeof window.TiptapEditor !== "undefined" &&
            window.TiptapEditor !== null &&
            typeof window.TiptapEditor.init === "function"
        );
    }

    function inject(src, onload, onerror) {
        var el = document.createElement("script");
        el.src = src;
        el.async = false; // deterministic order; tiptap-editor.js needs tiptap.min.js first
        el.onload = onload;
        el.onerror = onerror;
        document.head.appendChild(el);
    }

    function ready() {
        if (editorReady()) {
            return Promise.resolve(window.TiptapEditor);
        }
        if (loading) {
            return loading;
        }
        loading = new Promise(function (resolve, reject) {
            function stage(i) {
                if (i >= ASSETS.length) {
                    if (editorReady()) {
                        resolve(window.TiptapEditor);
                    } else {
                        reject(new Error("GoatKitEditor: assets loaded but TiptapEditor.init is missing"));
                    }
                    return;
                }
                inject(
                    ASSETS[i],
                    function () { stage(i + 1); },
                    function () { reject(new Error("GoatKitEditor: failed to load " + ASSETS[i])); }
                );
            }
            stage(0);
        });
        return loading;
    }

    function setContent(id, content) {
        // setEditorContent is a silent no-op when the instance does not exist
        // yet, so queue until BOTH the assets have loaded and init() has
        // completed for this editor (a later file-drop or draft restore must
        // win over the content passed at init time).
        if (!editorReady() || !initialized[id]) {
            pendingSet[id] = content;
            return;
        }
        window.TiptapEditor.setContent(id, content);
    }

    function init(opts) {
        opts = opts || {};
        var id = opts.id;
        if (!id) {
            return Promise.reject(new Error("GoatKitEditor.init: opts.id is required"));
        }
        if (!document.getElementById(id)) {
            return Promise.reject(new Error("GoatKitEditor.init: no container element #" + id));
        }
        var preExisting = !!known[id];
        return ready().then(function (TiptapEditor) {
            var inst = TiptapEditor.init(id, {
                mode: opts.mode || "edit",
                editorMode: opts.editorMode || "richtext",
                placeholder: opts.placeholder,
                content: opts.content || "",
                onUpdate: opts.onUpdate || null,
                imageUploadUrl: opts.imageUploadUrl || "",
                imageUrlPrompt: opts.imageUrlPrompt,
                linkUrlPrompt: opts.linkUrlPrompt
            });
            if (!inst) {
                return null;
            }
            initialized[id] = true;
            // Cache hit: the underlying init returned the old instance and
            // ignored this call's content — push it through instead.
            if (preExisting && opts.content !== undefined && opts.content !== null) {
                TiptapEditor.setContent(id, opts.content);
            }
            // set() calls made before the editor existed are applied last so
            // they win over the init-time content (draft restores, file drops).
            if (Object.prototype.hasOwnProperty.call(pendingSet, id)) {
                TiptapEditor.setContent(id, pendingSet[id]);
                delete pendingSet[id];
            }
            known[id] = true;
            return inst;
        });
    }

    window.GoatKitEditor = {
        ready: ready,
        isReady: editorReady,
        init: init,
        content: function (id) {
            return editorReady() && initialized[id] ? window.TiptapEditor.getContent(id) : "";
        },
        set: setContent,
        setMode: function (id, mode) {
            if (editorReady() && initialized[id] && window.TiptapEditor.setMode) {
                window.TiptapEditor.setMode(id, mode);
            }
        },
        destroy: function (id) {
            if (editorReady()) {
                window.TiptapEditor.destroy(id);
            }
            delete known[id];
            delete initialized[id];
            delete pendingSet[id];
        },
        insertText: function (id, text) {
            if (editorReady() && initialized[id] && window.TiptapEditor.insertText) {
                window.TiptapEditor.insertText(id, text);
            }
        }
    };
})();
