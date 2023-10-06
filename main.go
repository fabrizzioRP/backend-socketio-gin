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
	Campana   string
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

			client := Client{IdProject: mapData["id"], Campana: mapData["campana"]}

			// insert client to room.
			if err := s.Join(client.Campana); err != nil {
				return fmt.Errorf("client can't join to room")
			}

			// insert client into instance.
			connectionsClient[s.ID().String()] = client

			// emit message to room.
			msgNotification := fmt.Sprintf("new client connected: %s, in campaing: %s", s.ID().String(), client.Campana)
			websocket.To(client.Campana).Emit("join-notification", ser.String(msgNotification))

			// start streaming-data.
			go initStreamingData(websocket)
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
			s.Leave(client.Campana)

			// limpiamos la info del cliente.
			connectionsClient[s.ID().String()] = Client{}

			// start streaming-data.
			go initStreamingData(websocket)
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

/**/
func initStreamingData(ws *sio.ServerV4) {
	for {
		allCampaigns := getAllCampaigns()

		for _, c := range allCampaigns {
			msg := fmt.Sprintf("External streaming-data campaing: %s", c)
			ws.To(c).Emit("streamingData", ser.String(msg))
		}

		time.Sleep(1 * time.Second)
	}
}

/*
Esta funcion busca en la instancia clientes conectado
cuantas campañas hay conectadas y retorna un arreglo
de los nombres de las campañas.
*/
func getAllCampaigns() []string {
	campaignToStreaming := []string{}
	for _, client := range connectionsClient {
		exists := false
		for _, c := range campaignToStreaming {
			if c == client.Campana {
				exists = true
			}
		}

		if !exists {
			campaignToStreaming = append(campaignToStreaming, client.Campana)
		}
	}

	return campaignToStreaming
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
