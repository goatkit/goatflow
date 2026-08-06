// GoatKit Autocomplete module - generic suggestion lifecycle (data-gk-autocomplete)
(function () {
    if (window.GoatKitAutocompleteLoaded) return;
    window.GoatKitAutocompleteLoaded = true;
    const REGISTRY = {};
    const DEFAULTS = { minChars: 1, maxResults: 10, debounce: 120 };
    // Inputs whose next focus should NOT open the dropdown (e.g. after removing
    // a chip we refocus the field but don't want the suggestion list to pop open).
    const suppressOpen = new Set();
    const DBG = () => !!window.GK_DEBUG;
    function sanitizeJSON(text) {
        // Remove trailing commas before ] or }
        return text.replace(/,(\s*[\]}])/g, "$1");
    }
    function loadAllSeeds() {
        window.GoatKitSeeds = window.GoatKitSeeds || {};
        const scripts = document.querySelectorAll(
            'script[type="application/json"][data-gk-seed]',
        );
        scripts.forEach((sc) => {
            const type = sc.getAttribute("data-gk-seed");
            if (window.GoatKitSeeds[type]) return; // don't overwrite
            try {
                const raw = sc.textContent || "[]";
                const data = JSON.parse(sanitizeJSON(raw));
                if (Array.isArray(data)) {
                    window.GoatKitSeeds[type] = data;
                    if (DBG())
                        console.log("[GK-AUTO] loaded seed", type, data.length);
                } else {
                    console.warn("[GK-AUTO] seed not array", type);
                }
            } catch (err) {
                console.warn("[GK-AUTO] seed parse error", type, err.message);
            }
        });
    }
    function $(sel, ctx = document) {
        return ctx.querySelector(sel);
    }
    function $all(sel, ctx = document) {
        return Array.from(ctx.querySelectorAll(sel));
    }
    function debounce(fn, ms) {
        let t;
        return (...a) => {
            clearTimeout(t);
            t = setTimeout(() => fn(...a), ms);
        };
    }
    function parseNumber(v, def) {
        const n = parseInt(v, 10);
        return isNaN(n) ? def : n;
    }
    function compileTemplate(tpl, obj) {
        return tpl
            .replace(/{{\s*([\w.]+)\s*}}/g, (_, k) => {
                const v = obj[k];
                return v == null ? "" : String(v);
            })
            .replace(/\{\s*([\w.]+)\s*\}/g, (_, k) => {
                const v = obj[k];
                return v == null ? "" : String(v);
            });
    }
    function ensureList(input) {
        const id = input.getAttribute("aria-controls");
        if (!id) return null;
        let list = document.getElementById(id);
        if (!list) {
            list = document.createElement("div");
            list.id = id;
            list.className =
                "absolute z-10 mt-1 w-full bg-white dark:bg-gray-700 shadow-lg max-h-60 rounded-md py-1 text-base ring-1 ring-black ring-opacity-5 overflow-auto focus:outline-none sm:text-sm hidden";
            list.setAttribute("role", "listbox");
            input.parentNode.appendChild(list);
            if (DBG()) console.log("[GK-AUTO] created list container", id);
        }
        return list;
    }
    function openList(list) {
        if (!list) return;
        list.classList.remove("hidden");
        inputExpanded(list, true);
        dispatch(list, "open");
    }
    function closeList(list) {
        if (!list) return;
        list.classList.add("hidden");
        inputExpanded(list, false);
        dispatch(list, "close");
    }
    function inputExpanded(list, expanded) {
        const input = findInputForList(list);
        if (input) {
            input.setAttribute("aria-expanded", expanded ? "true" : "false");
            if (!expanded) input.removeAttribute("aria-activedescendant");
        }
    }
    function findInputForList(list) {
        return document.querySelector(`[aria-controls="${list.id}"]`);
    }
    function dispatch(el, name, detail = {}) {
        (findInputForList(el) || el).dispatchEvent(
            new CustomEvent(`gk:autocomplete:${name}`, {
                detail,
                bubbles: true,
            }),
        );
    }
    function seedData(type) {
        if (!window.GoatKitSeeds) return null;
        return window.GoatKitSeeds[type] || null;
    }
    function readInlineSeed(type) {
        const script = $(
            `script[type="application/json"][data-gk-seed="${type}"]`,
        );
        if (!script) return null;
        try {
            return JSON.parse(sanitizeJSON(script.textContent)) || null;
        } catch (err) {
            console.warn(
                "[GK-AUTO] inline seed parse error",
                type,
                err.message,
            );
            return null;
        }
    }
    function getDataSource(input) {
        return input.dataset.source || null;
    }
    async function fetchRemote(src, q) {
        const url = src.replace("{q}", encodeURIComponent(q));
        const r = await fetch(url, { credentials: "include" });
        if (!r.ok) return [];
        try {
            return await r.json();
        } catch (_) {
            return [];
        }
    }
    function highlight(list, idx) {
        const items = $all('[role="option"]', list);
        items.forEach((it, i) => {
            if (i === idx) {
                it.classList.add("gk-ta-active");
                it.setAttribute("aria-selected", "true");
                const input = findInputForList(list);
                if (input) input.setAttribute("aria-activedescendant", it.id);
            } else {
                it.classList.remove("gk-ta-active");
                it.removeAttribute("aria-selected");
            }
        });
    }
    function valueField(input) {
        return input.dataset.valueField || "login";
    }
    function displayTemplate(input) {
        return input.dataset.displayTemplate || "{{displayName}}";
    }
    function minChars(input) {
        return parseNumber(input.dataset.minChars, DEFAULTS.minChars);
    }
    function maxResults(input) {
        return parseNumber(input.dataset.maxResults, DEFAULTS.maxResults);
    }
    function isMultiple(input) {
        return (
            input.dataset.multiple === "true" ||
            input.hasAttribute("data-multiple")
        );
    }
    function selectedValues(input, state) {
        return state.selected || [];
    }
    function itemValue(input, item) {
        return item[valueField(input)] ?? item.login ?? item.id;
    }
    function chipsContainer(input) {
        const id = input.dataset.chipsTarget;
        if (id) {
            const el = document.getElementById(id);
            if (el) return el;
        }
        let wrap = input.parentNode.querySelector(
            ":scope > .gk-multiselect-chips",
        );
        if (!wrap) {
            wrap = document.createElement("div");
            wrap.className = "gk-multiselect-chips mt-2 flex flex-wrap gap-1";
            input.parentNode.appendChild(wrap);
        }
        return wrap;
    }
    function syncMultipleValue(input, selected) {
        const hid = input.dataset.multipleTarget || input.dataset.hiddenTarget;
        if (hid) {
            const h = document.getElementById(hid);
            if (h) {
                h.value = selected.join(",");
                h.dispatchEvent(new Event("change", { bubbles: true }));
            }
        }
    }
    function renderChips(input, state) {
        const selected = selectedValues(input, state);
        const wrap = chipsContainer(input);
        wrap.innerHTML = "";
        selected.forEach((val) => {
            const chip = document.createElement("span");
            chip.className =
                "gk-chip inline-flex items-center gap-1 pl-2 pr-1 py-0.5 rounded text-sm";
            chip.style.cssText =
                "background: var(--gk-primary-subtle); color: var(--gk-primary); border: 1px solid var(--gk-border-default);";
            chip.textContent = val;
            const rm = document.createElement("button");
            rm.type = "button";
            rm.setAttribute("aria-label", "Remove " + val);
            rm.innerHTML = "&times;";
            rm.style.cssText =
                "border:none;background:none;cursor:pointer;color:var(--gk-text-muted);padding:0 2px;font-size:1rem;line-height:1;";
            rm.addEventListener("click", (e) => {
                e.preventDefault();
                removeSelected(input, state, val);
            });
            chip.appendChild(rm);
            wrap.appendChild(chip);
        });
    }
    function removeSelected(input, state, val) {
        state.selected = (state.selected || []).filter(
            (v) => v !== val,
        );
        renderChips(input, state);
        syncMultipleValue(input, state.selected);
        // Refocus without popping the dropdown open (see focus handler).
        suppressOpen.add(input.id);
        input.focus();
    }
    function setMultipleState(input, values) {
        const state =
            REGISTRY[input.id] ||
            (REGISTRY[input.id] = {
                items: [],
                filtered: [],
                active: -1,
                loading: false,
                selected: [],
            });
        const list = Array.isArray(values)
            ? values
            : values
              ? String(values).split(",")
              : [];
        state.selected = [...new Set(list.filter(Boolean))];
        renderChips(input, state);
        syncMultipleValue(input, state.selected);
    }
    function setupInput(input) {
        if (REGISTRY[input.id]) return;
        if (!input.id) {
            input.id = `gk-ac-${Math.random().toString(36).slice(2)}`;
        }
        if (DBG())
            console.log(
                "[GK-AUTO] init",
                input.id,
                input.dataset.gkAutocomplete,
            );
        input.setAttribute("role", "combobox");
        input.setAttribute("aria-autocomplete", "list");
        input.setAttribute("aria-expanded", "false");
        const type = input.dataset.gkAutocomplete;
        const list = ensureList(input);
        if (!list) return;
        list.setAttribute("role", "listbox");
        let state = { items: [], filtered: [], active: -1, loading: false, selected: [] };
        REGISTRY[input.id] = state;
        const src = getDataSource(input);
        // Try to get seed data - if not found, try loading all seeds first (handles race conditions)
        let localSeed = seedData(type) || readInlineSeed(type);
        if (!localSeed && !src) {
            // Seeds might not be loaded yet - try loading them now
            loadAllSeeds();
            localSeed = seedData(type) || readInlineSeed(type);
        }
        if (localSeed) {
            if (DBG())
                console.log("[GK-AUTO] seed size", type, localSeed.length);
        } else if (!src) {
            console.warn("[GK-AUTO] no seed and no source for type", type);
        }
        const debounced = debounce(
            async () => {
                await refresh(input, state, src, localSeed);
            },
            parseNumber(input.dataset.debounce, DEFAULTS.debounce),
        );
        input.addEventListener("input", () => {
            state.active = -1;
            debounced();
        });
        input.addEventListener("focus", () => {
            // Some programmatic refocuses (chip removal) shouldn't pop the list open.
            if (suppressOpen.delete(input.id)) return;
            // trigger initial fetch on first char or empty if remote wants suggestions
            if (input.value.trim().length >= minChars(input)) debounced();
        });
        input.addEventListener("keydown", (e) => {
            if (list.classList.contains("hidden")) {
                if (e.key === "ArrowDown") {
                    // open and highlight first
                    debounced();
                    e.preventDefault();
                    return;
                }
            }
            if (e.key === "ArrowDown") {
                e.preventDefault();
                state.active = Math.min(
                    state.active + 1,
                    state.filtered.length - 1,
                );
                highlight(list, state.active);
            } else if (e.key === "ArrowUp") {
                e.preventDefault();
                state.active = Math.max(state.active - 1, 0);
                highlight(list, state.active);
            } else if (e.key === "Escape") {
                closeList(list);
            } else if (e.key === "Enter") {
                if (state.active > -1) {
                    e.preventDefault();
                    commitSelection(input, list, state.filtered[state.active]);
                    closeList(list);
                }
            }
        });
        // Close on blur if neither input nor list retains focus (delay to allow click selection)
        input.addEventListener("blur", () => {
            setTimeout(() => {
                const active = document.activeElement;
                if (active !== input && !list.contains(active)) closeList(list);
            }, 120);
        });
        document.addEventListener("click", (e) => {
            if (!input.contains(e.target) && !list.contains(e.target))
                closeList(list);
        });
    }
    function commitSelection(input, list, item) {
        if (!item) return;
        const val = itemValue(input, item);
        const display = compileTemplate(displayTemplate(input), item);

        if (!isMultiple(input)) {
            // Single-select: put the chosen display in the field and set one hidden value.
            const hiddenId = input.dataset.hiddenTarget;
            input.value = display;
            if (hiddenId) {
                const hidden = document.getElementById(hiddenId);
                if (hidden) {
                    hidden.value = val;
                    hidden.dispatchEvent(
                        new Event("change", { bubbles: true }),
                    );
                }
            }
            dispatch(list, "select", { value: val, display, data: item });
            return;
        }

        // Multi-select: append a chip, keep the field clear for more searches.
        const state = REGISTRY[input.id] || (REGISTRY[input.id] = { selected: [] });
        if (!state.selected) state.selected = [];
        if (state.selected.includes(val)) return; // already added
        state.selected.push(val);
        state.active = -1;
        renderChips(input, state);
        syncMultipleValue(input, state.selected);
        input.value = "";
        input.focus();
        dispatch(list, "select", { value: val, display, data: item });
    }
    async function refresh(input, state, src, localSeed) {
        const q = input.value.trim();
        const list = ensureList(input);
        if (!list) return;
        if (q.length < minChars(input)) {
            closeList(list);
            return;
        }
        let base = [];
        if (src) {
            base = await fetchRemote(src, q);
        } else if (localSeed && localSeed.length) {
            base = localSeed;
        } else {
            // Seed may have been loaded after setupInput - try to get it now
            const type = input.dataset.gkAutocomplete;
            const lateSeed = seedData(type) || readInlineSeed(type);
            if (lateSeed && lateSeed.length) {
                base = lateSeed;
                if (DBG())
                    console.log(
                        "[GK-AUTO] late seed load for",
                        type,
                        lateSeed.length,
                    );
            } else {
                if (DBG())
                    console.log("[GK-AUTO] abort refresh: no data source");
                closeList(list);
                return;
            }
        }
        const lower = q.toLowerCase();
        const sel = new Set(selectedValues(input, state));
        const filtered = base
            .filter((obj) => {
                if (sel.size && sel.has(itemValue(input, obj))) return false;
                return Object.values(obj).some(
                    (v) =>
                        typeof v === "string" &&
                        v.toLowerCase().includes(lower),
                );
            })
            .slice(0, maxResults(input));
        if (DBG())
            console.log("[GK-AUTO] refresh", {
                query: q,
                total: base.length,
                filtered: filtered.length,
            });
        state.filtered = filtered;
        list.innerHTML = "";
        if (filtered.length === 0) {
            list.innerHTML = `<div class="px-3 py-2 text-sm" style="color: var(--gk-text-muted);" data-empty>No results</div>`;
            openList(list);
            dispatch(list, "noresults");
            return;
        }
        filtered.forEach((item, i) => {
            const div = document.createElement("div");
            div.id = `${input.id}-opt-${i}`;
            div.setAttribute("role", "option");
            div.className = "gk-autocomplete-item cursor-pointer select-none relative py-2 pl-3 pr-9";
            div.style.cssText = "color: var(--gk-text-primary);";
            div.textContent = compileTemplate(displayTemplate(input), item);
            const login = item.login || item[valueField(input)] || "";
            if (login) {
                div.setAttribute("data-login", login);
                div.setAttribute("data-value", login);
            }
            div.addEventListener("mouseenter", () => {
                state.active = i;
                highlight(list, i);
            });
            div.addEventListener("mousedown", (e) => {
                e.preventDefault();
                commitSelection(input, list, item);
                closeList(list);
            });
            list.appendChild(div);
        });
        openList(list);
        state.active = 0;
        highlight(list, 0);
    }
    function initAll() {
        $all("input[data-gk-autocomplete]").forEach(setupInput);
    }
    document.addEventListener("DOMContentLoaded", () => {
        loadAllSeeds();
        initAll();
    });
    // Mutation observer for dynamically added inputs (guard if body not yet present)
    (function attachObserver() {
        if (!document.body) {
            // Retry shortly until body exists
            return setTimeout(attachObserver, 30);
        }
        new MutationObserver(() => initAll()).observe(document.body, {
            subtree: true,
            childList: true,
        });
    })();
    // Expose minimal API
    window.GoatKitAutocomplete = {
        refreshAll: () => {
            Object.keys(REGISTRY).forEach((id) => {
                const input = document.getElementById(id);
                if (input) input.dispatchEvent(new Event("input"));
            });
        },
        // Programmatically set/clear a multi-select autocomplete (tag chips).
        setMultiple: (id, values) => {
            const input = document.getElementById(id);
            if (input) setMultipleState(input, values);
        },
        clearMultiple: (id) => {
            const input = document.getElementById(id);
            if (input) setMultipleState(input, []);
        },
    };
})();
