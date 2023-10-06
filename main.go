package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	sio "github.com/njones/socketio"
	ser "github.com/njones/socketio/serialize"
)

type Client struct {
	IdProject string
	Project   string
}

var connectionsClient = make(map[string]Client)

// Define our custom event callback
type CustomWrap func([]interface{}) error

// Callback(...interface{}) error
func (cw CustomWrap) Callback(allValues ...interface{}) error { return cw(allValues) }

/*
Run program.
*/
func main() { initializeServerHTTP(initializeWebSocket()) }

/*
Logica del proceso de eventos del websocket
*/
func initializeWebSocket() *sio.ServerV4 {
	websocket := sio.NewServerV4()

	websocket.OnConnect(func(s *sio.SocketV4) error {
		socketId := s.ID().String()
		log.Println("Cliente Conectado:", socketId)
		connectionsClient[socketId] = Client{}

		/*** Evento de unir a una sola. ***/
		websocket.On("joinRoom", CustomWrap(func(allValues []interface{}) error {
			log.Println("============== JOIN ROOM ==============")
			data := allValues[0].(string)

			_, ok := connectionsClient[s.ID().String()]
			if !ok {
				return fmt.Errorf("client not found in connectionsClient")
			}

			mapData := parseDataFromClient(data)

			client := Client{IdProject: mapData["id"], Project: mapData["projectName"]}

			// insert client to room.
			if err := s.Join(client.Project); err != nil {
				return fmt.Errorf("client can't join to room")
			}

			// insert client into instance.
			connectionsClient[s.ID().String()] = client

			// emit message to room.
			msgNotification := fmt.Sprintf("new client connected: %s, in campaing: %s", s.ID().String(), client.Project)
			websocket.To(client.Project).Emit("join-notification", ser.String(msgNotification))

			// Se sabe que tenemos cliente hacemos la emicion de la data externa.
			go func() {
				for {
					externalData := fmt.Sprintf("External streaming-data campaing: %s || for client: %s", client.Project, s.ID().String())
					websocket.To(client.Project).Emit("streamingData", ser.String(externalData))
					time.Sleep(1 * time.Second)
				}
			}()

			return nil
		}))

		/*** Evento de salir de la sala. ***/
		websocket.On("leaveRoom", CustomWrap(func(_ []interface{}) error {
			log.Println("============== LEAVE ROOM ==============")
			// buscar al cliente en la instancia.
			client, exists := connectionsClient[s.ID().String()]
			if !exists {
				return fmt.Errorf("client no found")
			}

			// seteamos el room.
			s.Leave(client.Project)

			// limpiamos la info del cliente.
			connectionsClient[s.ID().String()] = Client{}
			return nil
		}))

		/*** Evento de disconnect. ***/
		websocket.OnDisconnect(func(reason string) {
			log.Println("============== DISCONNECT ==============")
			log.Printf("Conexion cerrada, cliente: %s, reason: %s", s.ID().String(), reason)
			delete(connectionsClient, s.ID().String())
		})

		return nil
	})

	return websocket
}

/*
Configuraciones Server Http con Gin.
*/
func initializeServerHTTP(serverSocket *sio.ServerV4) {
	gin.SetMode(gin.ReleaseMode)
	serverHttp := gin.New()

	serverHttp.Use(cors.Default())
	serverHttp.StaticFile("/", "./public/index.html")

	serverHttp.GET("/socket.io/", ginSocketIOServerWrapper(serverSocket))

	log.Println("server running on port 3000")
	log.Fatal(serverHttp.Run(":3000"))
}

/*
Decorator para implementar socketio como handler.
*/
func ginSocketIOServerWrapper(serverSocket *sio.ServerV4) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// log.Println("Method GET /socket.io/*any")
		// log.Println("Is the request from websocket?", ctx.IsWebsocket())
		if ctx.IsWebsocket() {
			serverSocket.ServeHTTP(ctx.Writer, ctx.Request)
		} else {
			_, _ = ctx.Writer.WriteString("===not websocket request===")
		}
	}
}

/*
Parse la data proveniente del cliente.
*/
func parseDataFromClient(data string) map[string]string {
	mapData := make(map[string]string)
	err := json.Unmarshal([]byte(data), &mapData)
	if err != nil {
		log.Fatal(err.Error())
	}

	return mapData
}
