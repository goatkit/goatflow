## i18n Form Macros & Customer Info Panel Rollout

### Macros
Defined in `templates/macros/forms.pongo2`:
- `i18n_text(name, base_key, value, required, placeholder_key, help_key, min_length, attrs)`
- `i18n_select(name, base_key, options, selected, required, help_key, attrs)` (expects slice of objects with `value` and `label`)
- `i18n_textarea(name, base_key, required, placeholder_key, help_key, min_length, attrs)`
- `mandatory_note(key)`

Key convention (examples):
```
tickets.subject -> label
tickets.subject.placeholder
tickets.subject.help
```
Missing keys fall back: current lang -> English inline map/JSON -> humanized last segment.

### Adding a New Field
1. Choose base key (`module.field`).
2. Add English + other locale entries.
3. Use macro: `{{ macros.forms.i18n_text("field", "module.field", form.field, true) }}`.

### Customer Info Panel
Endpoint: `GET /tickets/customer-info/:login` returns a partial.
Partials:
- `partials/tickets/customer_info.pongo2`
- `partials/tickets/customer_info_unregistered.pongo2`

Frontend trigger added in `goatkit-typeahead.js`:
- On hidden `customer_user_id` change -> fetch panel.
- On blur of `customer_user_input` with email pattern and no hidden value -> fetch unregistered panel.

To reuse on other forms:
```
<div id="customer-info-panel" class="hidden ..."></div>
<input id="customer_user_input" data-gk-autocomplete="customer-user" data-hidden-target="customer_user_id" ...>
<input type="hidden" id="customer_user_id">
```
JS hook auto-detects by IDs.

### Unregistered Emails
If no row in `customer_users`, the unregistered template renders minimal panel with email + note.

### Open Tickets Count
Query excludes `state IN ('closed','resolved')`.

### Adding More Fields to Panel
Extend SQL select + partial template; ensure keys exist under `forms.customer_info.*`.

### Validation Integration
Macros add `data-msg-required-key` / `data-msg-minlength-key` for `goatkit-validate.js`.

### Future Enhancements
- Cache customer info responses (ETag / client-side memo) if performance needed.
- Add company link or modal.
- Add avatar/Gravatar if available.

### Checklist for New Forms
- [ ] Insert mandatory note (or call `mandatory_note`).
- [ ] Use macros for all translatable fields.
- [ ] Provide locale keys in `en.json` and `de.json`.
- [ ] Add `customer-info-panel` container if customer selection present.
- [ ] Ensure IDs match (`customer_user_input`, `customer_user_id`).
