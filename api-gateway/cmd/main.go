package main

import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/miladhzzzz/milx-cloud-init/api-gateway/config"
	"github.com/miladhzzzz/milx-cloud-init/api-gateway/controllers"
	"github.com/miladhzzzz/milx-cloud-init/api-gateway/internal/trigger-grpc"
	"github.com/miladhzzzz/milx-cloud-init/api-gateway/routes"
	"github.com/miladhzzzz/milx-cloud-init/api-gateway/services"
	"github.com/miladhzzzz/milx-cloud-init/api-gateway/utils"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	cnf, _                = config.ReadConfig()
	auditUrl              = "http://localhost:8080"
	serviceName           = "persys-api-gateway"
	mySuperSecretPassword = "unicornsAreAwesome"
	webhookUrl            = cnf.WebHookURL    // Demo deployment on azure
	webhookSecret         = cnf.WebHookSecret // SECURITY ISSUE *** CHANGE THIS!

	server      *gin.Engine
	ctx         context.Context
	mongoclient *mongo.Client

	authCollection      *mongo.Collection
	authService         services.AuthService
	AuthController      controllers.AuthController
	AuthRouteController routes.AuthRouteController

	//👇 Create the Github Variables
	githubService         services.GithubService
	GithubController      controllers.GithubController
	GithubCollection      *mongo.Collection
	GithubRouteController routes.GithubRouteController
)

func init() {
	// initializing audit service
	utils.SendLogMessage(auditUrl, utils.LogMessage{
		Microservice: "api-gateway",
		Level:        "Info",
		Message:      "api gateway init",
		Timestamp:    time.Now(),
	})
	//cnf, _ = config.ReadConfig()
	// create a log file
	logFile, _ := os.Create("api-gateway-http.log")

	ctx = context.TODO()

	// Connect to MongoDB
	mongoconn := options.Client().ApplyURI(cnf.MongoURI)
	mongoclient, err := mongo.Connect(ctx, mongoconn)

	if err != nil {
		utils.LogError(err.Error())
		panic(err)
	}

	if err := mongoclient.Ping(ctx, readpref.Primary()); err != nil {
		utils.LogError(err.Error())
		panic(err)
	}

	fmt.Println("MongoDB successfully connected...")

	// Collections
	GithubCollection = mongoclient.Database("api-gateway").Collection("repos")
	authCollection = mongoclient.Database("api-gateway").Collection("users")
	githubService = services.NewGithubService(GithubCollection, ctx)
	authService = services.NewAuthService(authCollection, ctx)
	AuthController = controllers.NewAuthController(authService, ctx, githubService, authCollection)
	AuthRouteController = routes.NewAuthRouteController(AuthController)

	GithubController = controllers.NewGithubController(authService, ctx, githubService, GithubCollection)
	GithubRouteController = routes.NewGithubRouteController(GithubController)

	//// 👇 Instantiate the Constructors
	//postCollection = mongoclient.Database("golang_mongodb").Collection("posts")
	//postService = services.NewPostService(postCollection, ctx)
	//PostController = controllers.NewPostController(postService)
	//PostRouteController = routes.NewPostControllerRoute(PostController)

	server = gin.Default()
	server.Use(gin.LoggerWithWriter(logFile))

}

func main() {

	//cleanup := opentelemtry.InitTracer()
	//
	//	//defer errorhandler.ErrHandler()
	//
	//defer cleanup(context.Background())

	//defer mongoclient.Disconnect(ctx)

	//// 👇 Instantiate event processor

	// starting grpc trigger mechanism that calls events-manager service over gRPC
	go trigger_grpc.StartgRPCtrigger()
	// starting gin http server
	startGinServer()
	//startGrpcServer(config)

}

func startGinServer() {

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:8551"}
	corsConfig.AllowCredentials = true

	server.Use(cors.New(corsConfig))
	server.Use(otelgin.Middleware(serviceName))

	router := server.Group("")
	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "success", "message": "value"})
	})

	AuthRouteController.AuthRoute(router)
	GithubRouteController.GithubRoute(router)
	// 👇 Post Route
	//PostRouteController.PostRoute(router)
	log.Fatal(server.Run(cnf.HttpAddr))
}
