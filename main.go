package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Cliente representa un cliente conectado
type Client struct {
	conn *websocket.Conn // Conexión WebSocket del cliente
}

var (
	clients   = make(map[*Client]bool) // Lista de clientes conectados
	broadcast = make(chan string)      // Canal para mensajes de difusión
	mutex     = &sync.Mutex{}          // Mutex para proteger el acceso concurrente a la lista de clientes
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Permitir conexiones desde cualquier origen (útil para desarrollo, pero asegúrate de restringirlo en producción)
		},
	}
)

func main() {
	// Configuramos el manejador de la ruta /ws para manejar conexiones WebSocket
	http.HandleFunc("/ws", handleConnections)

	// Configuramos el manejador de la ruta raíz para servir la interfaz web
	http.HandleFunc("/", handleHome)

	// Lanzamos una goroutine para manejar los mensajes de difusión
	go handleMessages()

	// Iniciamos el servidor en el puerto 8080
	fmt.Println("Servidor iniciado en :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic("Error iniciando el servidor: " + err.Error())
	}
}

// handleHome sirve la página HTML principal
func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, "index.html")
}

// handleConnections maneja las conexiones WebSocket entrantes
func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Actualizamos la conexión HTTP a una conexión WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error al actualizar a WebSocket:", err)
		return
	}
	defer conn.Close() // Aseguramos que la conexión se cierre cuando termine la función

	// Creamos un nuevo cliente y lo agregamos a la lista de clientes conectados
	client := &Client{conn: conn}
	mutex.Lock()
	clients[client] = true
	mutex.Unlock()

	// Mantenemos la conexión activa con pings periódicos
	go func() {
		for {
			time.Sleep(30 * time.Second) // Enviar un ping cada 30 segundos
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				fmt.Println("Error enviando ping:", err)
				mutex.Lock()
				delete(clients, client)
				mutex.Unlock()
				return
			}
		}
	}()

	// Escuchamos mensajes enviados por el cliente
	for {
		// Leer mensaje desde el cliente como texto plano
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Error leyendo mensaje:", err)
			// Si ocurre un error, eliminamos al cliente de la lista y cerramos la conexión
			mutex.Lock()
			delete(clients, client)
			mutex.Unlock()
			break
		}
		// Enviamos el mensaje al canal de difusión
		broadcast <- string(msg)
	}
}

// handleMessages maneja los mensajes enviados al canal de difusión
func handleMessages() {
	for {
		// Escuchamos nuevos mensajes del canal de difusión
		msg := <-broadcast
		mutex.Lock()
		for client := range clients {
			// Enviar mensaje a todos los clientes conectados
			if err := client.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				fmt.Println("Error enviando mensaje:", err)
				// Si ocurre un error, cerramos la conexión del cliente y lo eliminamos de la lista
				client.conn.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}
