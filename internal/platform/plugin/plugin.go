// Package plugin re-exports the public plugin types from pkg/plugin.
//
// This allows internal code to continue importing "internal/platform/plugin" unchanged
// while the canonical type definitions live in the public pkg/plugin package,
// accessible to external plugin authors.
package plugin

import (
	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// Type aliases — these are identical to the pkg/plugin types.
// Existing internal code continues to work without import changes.

type Plugin = pkgplugin.Plugin
type GKRegistration = pkgplugin.GKRegistration
type RouteSpec = pkgplugin.RouteSpec
type MenuItemSpec = pkgplugin.MenuItemSpec
type WidgetSpec = pkgplugin.WidgetSpec
type JobSpec = pkgplugin.JobSpec
type TemplateSpec = pkgplugin.TemplateSpec
type I18nSpec = pkgplugin.I18nSpec
type ErrorCodeSpec = pkgplugin.ErrorCodeSpec
type HostAPI = pkgplugin.HostAPI
type ResourceRequest = pkgplugin.ResourceRequest
type Permission = pkgplugin.Permission
type ResourcePolicy = pkgplugin.ResourcePolicy
type CascadeSpec = pkgplugin.CascadeSpec
type CustomFieldSpec = pkgplugin.CustomFieldSpec
type CustomFieldFilter = pkgplugin.CustomFieldFilter
type FileInfo = pkgplugin.FileInfo
type ArticleAttachment = pkgplugin.ArticleAttachment
type PdfRenderOptions = pkgplugin.PdfRenderOptions
type UISpec = pkgplugin.UISpec
type UIRouteSpec = pkgplugin.UIRouteSpec
type UINavSpec = pkgplugin.UINavSpec
type UINavItem = pkgplugin.UINavItem
type UIBrandingSpec = pkgplugin.UIBrandingSpec
type UIAuthSpec = pkgplugin.UIAuthSpec
type UIPWASpec = pkgplugin.UIPWASpec
type MCPToolSpec = pkgplugin.MCPToolSpec

// DefaultResourcePolicy re-exports the default policy constructor.
var DefaultResourcePolicy = pkgplugin.DefaultResourcePolicy
