// Package base is the engine of Viva's page model: the element tree, layout,
// input and focus routing, control state and behaviour, and the hooks through
// which a styled layer paints it. It depends on gfx, style, text, field, keymap
// and window, and never on components, ui or app.
//
// Applications use package ui; base is published because two kinds of code
// compose it from outside package ui:
//
//   - the styled layer (package ui) paints base's elements through the hooks in
//     this package, and
//   - a recipe copied into an application with `viva add` is ordinary source in
//     that application, so it imports base directly for the state and the
//     measurements it reads.
//
// base is not the application authoring model: a page is declared with ui.C.
// The exported accessors whose documentation calls them "bridges" exist only for
// the painters above; they are not part of the developer-facing API.
package base
