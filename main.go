package main

import (
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	sio "github.com/njones/socketio"
	ser "github.com/njones/socketio/serialize"
)

func main() {
	// initialize Websocket.
	serverSocket := sio.NewServerV4()

	serverSocket.OnConnect(func(s *sio.SocketV4) error {
		log.Println("Cliente Conectado:", s.ID())

		// emit
		s.Emit("greeting", ser.String("can you hear me?"))

		return nil
	})

	serverSocket.OnDisconnect(func(s string) {
		log.Println("Conexion cerrada:", s)
	})

	// initialize Gin.
	gin.SetMode(gin.ReleaseMode)
	serverHttp := gin.New()

	serverHttp.Use(cors.Default())
	serverHttp.StaticFile("/", "./public/index.html")

	serverHttp.GET("/socket.io/", ginSocketIOServerWrapper(serverSocket))

	log.Println("server running on port 3000")
	log.Fatal(serverHttp.Run(":3000"))
}

func ginSocketIOServerWrapper(serverSocket *sio.ServerV4) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		log.Println("Method GET /socket.io/*any")
		log.Println("Is the request from websocket?", ctx.IsWebsocket())
		if ctx.IsWebsocket() {
			serverSocket.ServeHTTP(ctx.Writer, ctx.Request)
		} else {
			_, _ = ctx.Writer.WriteString("===not websocket request===")
		}
	}
}
