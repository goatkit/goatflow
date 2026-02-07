package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Agent token handlers
	routing.RegisterHandler("HandleListTokens", HandleListTokens)
	routing.RegisterHandler("HandleCreateToken", HandleCreateToken)
	routing.RegisterHandler("HandleRevokeToken", HandleRevokeToken)
	routing.RegisterHandler("HandleGetScopes", HandleGetScopes)

	// Admin token handlers (agents)
	routing.RegisterHandler("HandleAdminListAllTokens", HandleAdminListAllTokens)
	routing.RegisterHandler("HandleAdminRevokeToken", HandleAdminRevokeToken)
	routing.RegisterHandler("HandleAdminListUserTokens", HandleAdminListUserTokens)
	routing.RegisterHandler("HandleAdminCreateUserToken", HandleAdminCreateUserToken)
	routing.RegisterHandler("HandleAdminRevokeUserToken", HandleAdminRevokeUserToken)

	// Admin token handlers (customers)
	routing.RegisterHandler("HandleAdminListCustomerTokens", HandleAdminListCustomerTokens)
	routing.RegisterHandler("HandleAdminCreateCustomerToken", HandleAdminCreateCustomerToken)
	routing.RegisterHandler("HandleAdminRevokeCustomerToken", HandleAdminRevokeCustomerToken)

	// Customer token handlers
	routing.RegisterHandler("HandleCustomerListTokens", HandleCustomerListTokens)
	routing.RegisterHandler("HandleCustomerCreateToken", HandleCustomerCreateToken)
	routing.RegisterHandler("HandleCustomerRevokeToken", HandleCustomerRevokeToken)
}
