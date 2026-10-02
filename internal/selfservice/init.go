package selfservice

import "github.com/goatkit/goatflow/internal/platform/routing"

// Handler names referenced by routes/selfservice.yaml.
func init() {
	lost := lostPasswordEnabled
	reg := registrationEnabled

	routing.RegisterHandler("handleAgentForgotPasswordPage", requireFeature(lost, forgotPasswordPage(agentPortal)))
	routing.RegisterHandler("handleAgentForgotPasswordSubmit", requireFeature(lost, forgotPasswordSubmit(agentPortal)))
	routing.RegisterHandler("handleAgentResetPasswordPage", requireFeature(lost, resetPasswordPage(agentPortal)))
	routing.RegisterHandler("handleAgentResetPasswordSubmit", requireFeature(lost, resetPasswordSubmit(agentPortal)))

	routing.RegisterHandler("handleCustomerForgotPasswordPage", requireFeature(lost, forgotPasswordPage(customerPortal)))
	routing.RegisterHandler("handleCustomerForgotPasswordSubmit", requireFeature(lost, forgotPasswordSubmit(customerPortal)))
	routing.RegisterHandler("handleCustomerResetPasswordPage", requireFeature(lost, resetPasswordPage(customerPortal)))
	routing.RegisterHandler("handleCustomerResetPasswordSubmit", requireFeature(lost, resetPasswordSubmit(customerPortal)))

	routing.RegisterHandler("handleCustomerRegisterPage", requireFeature(reg, registerPage))
	routing.RegisterHandler("handleCustomerRegisterSubmit", requireFeature(reg, registerSubmit))
	routing.RegisterHandler("handleCustomerRegisterCompletePage", requireFeature(reg, registerCompletePage))
	routing.RegisterHandler("handleCustomerRegisterCompleteSubmit", requireFeature(reg, registerCompleteSubmit))
}
