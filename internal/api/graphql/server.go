package graphql

import (
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// GraphQLServer wraps the GraphQL handler
type GraphQLServer struct {
	resolver *Resolver
	handler  *handler.Server
}

// NewGraphQLServer creates a new GraphQL server
func NewGraphQLServer(
	userService *service.UserService,
	ticketService *service.TicketService,
	queueService *service.QueueService,
	workflowService *service.WorkflowService,
	authService *service.AuthService,
	searchService *service.SearchService,
	reportService *service.ReportService,
) *GraphQLServer {
	resolver := NewResolver(
		userService,
		ticketService,
		queueService,
		workflowService,
		authService,
		searchService,
		reportService,
	)

	// Create GraphQL handler
	srv := handler.NewDefaultServer(&ExecutableSchema{
		resolvers: resolver,
	})

	return &GraphQLServer{
		resolver: resolver,
		handler:  srv,
	}
}

// ExecutableSchema implements the GraphQL executable schema
type ExecutableSchema struct {
	resolvers *Resolver
}

// Handler returns the GraphQL HTTP handler
func (s *GraphQLServer) Handler() gin.HandlerFunc {
	return gin.WrapH(s.handler)
}

// PlaygroundHandler returns the GraphQL playground handler
func (s *GraphQLServer) PlaygroundHandler() gin.HandlerFunc {
	h := playground.Handler("GraphQL playground", "/api/v1/graphql")
	return gin.WrapH(h)
}

// RegisterRoutes registers GraphQL routes
func (s *GraphQLServer) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/graphql", s.Handler())
	r.GET("/graphql", s.PlaygroundHandler())
}