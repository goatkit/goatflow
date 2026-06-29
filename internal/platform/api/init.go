package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	routing.RegisterHandler("HandleLoginAPI", HandleLoginAPI)
	routing.RegisterHandler("HandleRefreshTokenAPI", HandleRefreshTokenAPI)
	routing.RegisterHandler("HandleLogoutAPI", HandleLogoutAPI)
	routing.RegisterHandler("HandleRegisterAPI", HandleRegisterAPI)
	routing.RegisterHandler("HandleUserMeAPI", HandleUserMeAPI)
	routing.RegisterHandler("HandleListUsersAPI", HandleListUsersAPI)
	routing.RegisterHandler("HandleGetUserAPI", HandleGetUserAPI)
	routing.RegisterHandler("HandleCreateUserAPI", HandleCreateUserAPI)
	routing.RegisterHandler("HandleUpdateUserAPI", HandleUpdateUserAPI)
	routing.RegisterHandler("HandleDeleteUserAPI", HandleDeleteUserAPI)
}
