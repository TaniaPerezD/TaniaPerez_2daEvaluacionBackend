package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"io"

	"github.com/hudl/fargo"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// httpClient con timeout configurable para pings
var httpClient = &http.Client{
	Timeout: 10 * time.Second, // ajusta aquí el timeout deseado
}

// getOutboundIP intenta averiguar la IP local “real” (no loopback)
func getOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func main() {
	// Instancia PocketBase
	app := pocketbase.New()

	// Variables de configuración (pueden venir de entorno)
	eurekaURL := os.Getenv("EUREKA_URL")
	if eurekaURL == "" {
		eurekaURL = "http://172.25.136.15:8761/eureka"
	}
	appName := os.Getenv("EUREKA_APP")
	if appName == "" {
		appName = "POCKETBASE-SERVER"
	}
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8090"
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	// Detectar IP y hostname
	ip, err := getOutboundIP()
	if err != nil {
		log.Printf("⚠️ No pude auto-detectar IP: %v, usaré loopback", err)
		ip = "127.0.0.1"
	}
	hostname, _ := os.Hostname()

	// Hook de bootstrap: registro inicial en Eureka
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		ec := fargo.NewConn(eurekaURL)
		client := &ec

		inst := &fargo.Instance{
			HostName:       hostname,
			Port:           port,
			App:            appName,
			IPAddr:         ip,
			VipAddress:     appName,
			Status:         fargo.UP,
			DataCenterInfo: fargo.DataCenterInfo{Name: fargo.MyOwn},
		}

		if err := client.RegisterInstance(inst); err != nil {
			log.Printf("⚠️ Eureka registro fallido (omitido): %v", err)
		} else {
			log.Println("✅ Registrado en Eureka")
		}

		return e.Next()
	})

	// Hook de serve: tus rutas
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/myapp/settings", func(e *core.RequestEvent) error {

			return e.JSON(http.StatusOK, map[string]bool{"success": true})
		}).Bind(apis.RequireAuth())

		se.Router.POST("/api/custom/email", func(e *core.RequestEvent) error {
			// logica de jhes
			//comprobar que el usuario sea admin y exista la persona a la que quiere enviar un correo
			// enviar el correo al microservicio de jhes
			return e.JSON(http.StatusOK, map[string]bool{"success": true})
		}).Bind(apis.RequireAuth())

		se.Router.GET("/api/custom/emails", func(e *core.RequestEvent) error {
			// logica de jhes
			//comprobar que el usuario sea admin y existan las personas a las que quiere enviar un correo
			// enviar el correo al microservicio de jhes
			return e.JSON(http.StatusOK, map[string]bool{"success": true})
		}).Bind(apis.RequireAuth())

		//microservicio de tania

		se.Router.POST("/api/custom/create-meeting", func(e *core.RequestEvent) error {
			// 1. Autenticación
			user := e.Auth
			if user == nil {
				return apis.NewUnauthorizedError("Usuario no autenticado", nil)
			}

			// 2. Autorización
			role, _ := user.Get("rol").(string)
			if role != "director" {
				return apis.NewForbiddenError("Solo directores pueden crear reuniones", nil)
			}

			// 3. Leer body en struct
			var body struct {
				Agenda      string `json:"agenda"`
				Duration    int    `json:"duration"`
				PreSchedule bool   `json:"pre_schedule"`
				Settings    struct {
					EmailNotification bool `json:"email_notification"`
					MeetingInvitees   []struct {
						Email string `json:"email"`
					} `json:"meeting_invitees"`
				} `json:"settings"`
				StartTime time.Time `json:"start_time"`
				Timezone  string    `json:"timezone"`
				Type      int       `json:"type"`
			}
			if err := e.BindBody(&body); err != nil {
				return e.BadRequestError("payload inválido", err)
			}

			// Extraer email del estudiante
			studentEmail := ""
			if len(body.Settings.MeetingInvitees) > 0 {
				studentEmail = body.Settings.MeetingInvitees[0].Email
			}
			if studentEmail == "" {
				return e.BadRequestError("no se proporcionó email de estudiante", nil)
			}

			// 4. Serializar y enviar al microservicio
			payload, err := json.Marshal(body)
			if err != nil {
				log.Printf("Error serializando body: %v", err)
				return apis.NewInternalServerError("Error interno al preparar la solicitud", err)
			}

			email, _ := user.Get("email").(string)
			meetingsServiceURL := fmt.Sprintf("http://localhost:8019/api/meetings/create/%s", email)
			microResp, err := httpClient.Post(
				meetingsServiceURL,
				"application/json",
				bytes.NewReader(payload),
			)
			if err != nil {
				log.Printf("Error llamando al microservicio: %v", err)
				return apis.NewInternalServerError("Error al crear la reunión en el microservicio", err)
			}
			defer microResp.Body.Close()

			microBody, err := io.ReadAll(microResp.Body)
			if err != nil {
				log.Printf("Error leyendo respuesta del microservicio: %v", err)
				return apis.NewInternalServerError("Error interno al leer respuesta", err)
			}
			if microResp.StatusCode < 200 || microResp.StatusCode >= 300 {
				return e.Stream(microResp.StatusCode, microResp.Header.Get("Content-Type"), bytes.NewReader(microBody))
			}

			// 5. Deserializar respuesta del microservicio
			var meetingData map[string]interface{}
			if err := json.Unmarshal(microBody, &meetingData); err != nil {
				log.Printf("Error deserializando respuesta: %v", err)
				return e.Stream(microResp.StatusCode, microResp.Header.Get("Content-Type"), bytes.NewReader(microBody))
			}

			// 6. Buscar estudiante por email
			studentRecord, err := e.App.FindFirstRecordByData("usuarios", "email", studentEmail)
			if err != nil {
				return apis.NewNotFoundError("Estudiante no encontrado", err)
			}

			// 7. Guardar reunión en PocketBase
			meetingsColl, err := e.App.FindCollectionByNameOrId("reuniones")
			if err != nil {
				log.Printf("Colección 'reuniones' no encontrada: %v", err)
				return apis.NewInternalServerError("Error interno: colección no encontrada", err)
			}

			rec := core.NewRecord(meetingsColl)
			rec.Set("host", user.Id)
			rec.Set("estudiante", studentRecord.Id)
			rec.Set("tipo_reunion", "consulta")
			rec.Set("fecha", meetingData["start_time"])
			rec.Set("duracion", meetingData["duration"])
			rec.Set("link", meetingData["join_url"])
			rec.Set("id_zoom", meetingData["id"])
			rec.Set("presencial", false)

			if err := e.App.Save(rec); err != nil {
				log.Printf("Error guardando reunión: %v", err)
				return apis.NewInternalServerError("Error al guardar la reunión", err)
			}

			// 8. Responder al cliente
			return e.JSON(http.StatusCreated, map[string]interface{}{
				"pocketbase_record_id": rec.Id,
				"microservice_data":    meetingData,
			})
		}).Bind(apis.RequireAuth())

		se.Router.PATCH("/api/custom/update-meeting/:id", func(e *core.RequestEvent) error {
			user := e.Auth
			if user == nil {
				return apis.NewUnauthorizedError("No autenticado", nil)
			}
			role, _ := user.Get("rol").(string)
			if role != "director" {
				return apis.NewForbiddenError("No autorizado", nil)
			}

			meetingId := e.Request.PathValue("id")

			// Obtener la reunión de PocketBase
			record, err := e.App.FindRecordById("reuniones", meetingId)
			if err != nil {
				return apis.NewNotFoundError("Reunión no encontrada", err)
			}

			// Leer el cuerpo con los nuevos datos
			var updateData map[string]interface{}
			if err := e.BindBody(&updateData); err != nil {
				return e.BadRequestError("JSON inválido", err)
			}

			// Llamar al microservicio para actualizar la reunión de Zoom
			zoomId := record.GetInt("id_zoom")
			payload, _ := json.Marshal(updateData)

			req, err := http.NewRequest("PATCH", fmt.Sprintf("http://localhost:8019/api/meetings/update/%d", zoomId), bytes.NewReader(payload))
			if err != nil {
				return apis.NewInternalServerError("Error creando solicitud PATCH", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := httpClient.Do(req)
			if err != nil {
				return apis.NewInternalServerError("Error al llamar al microservicio", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				return apis.NewInternalServerError(fmt.Sprintf("Error del microservicio: %s", body), nil)
			}

			// Actualizar también en PocketBase (si quieres reflejar cambios locales)
			for key, val := range updateData {
				record.Set(key, val)
			}
			if err := e.App.Save(record); err != nil {
				return apis.NewInternalServerError("Error al guardar cambios locales", err)
			}

			return e.JSON(http.StatusOK, record)
		}).Bind(apis.RequireAuth())

		se.Router.DELETE("/api/custom/delete-meeting/:id", func(e *core.RequestEvent) error {
			user := e.Auth
			if user == nil {
				return apis.NewUnauthorizedError("No autenticado", nil)
			}
			role, _ := user.Get("rol").(string)
			if role != "director" {
				return apis.NewForbiddenError("No autorizado", nil)
			}

			meetingId := e.Request.PathValue("id")

			// Obtener el registro en PocketBase
			record, err := e.App.FindRecordById("reuniones", meetingId)
			if err != nil {
				return apis.NewNotFoundError("Reunión no encontrada", err)
			}

			// Llamar al microservicio para eliminar también en Zoom
			zoomId := record.GetInt("id_zoom")
			req, _ := http.NewRequest(http.MethodDelete,
				fmt.Sprintf("http://localhost:8019/api/meetings/delete/%d", zoomId),
				nil)

			resp, err := httpClient.Do(req)
			if err != nil || resp.StatusCode >= 400 {
				return apis.NewInternalServerError("Error al eliminar en microservicio", err)
			}

			// Eliminar en PocketBase
			if err := e.App.Delete(record); err != nil {
				return apis.NewInternalServerError("Error al eliminar localmente", err)
			}

			return e.JSON(http.StatusOK, map[string]string{
				"message": "Reunión eliminada correctamente",
			})
		}).Bind(apis.RequireAuth())

		return se.Next()
	})

	// Hook de terminate: deregistro de Eureka
	app.OnTerminate().BindFunc(func(te *core.TerminateEvent) error {
		log.Println("⚠️ PocketBase terminando, desregistrando de Eureka...")
		ec := fargo.NewConn(eurekaURL)
		client := &ec
		if err := client.DeregisterInstance(&fargo.Instance{HostName: hostname, App: appName}); err != nil {
			log.Printf("⚠️ Error desregistrando en Eureka: %v", err)
		} else {
			log.Println("🗑️ Desregistrado de Eureka exitosamente")
		}
		return te.Next()
	})

	// Cron job: heartbeat y ping a instancias cada 5 minutos
	app.Cron().MustAdd("eureka-health", "*/5 * * * *", func() {
		ec := fargo.NewConn(eurekaURL)
		client := &ec
		inst := &fargo.Instance{HostName: hostname, App: appName}

		// Heartbeat silencioso
		_ = client.HeartBeatInstance(inst)

		// Ping a todas las instancias registradas
		apps, err := client.GetApps()
		if err != nil {
			log.Printf("⚠️ Error leyendo apps de Eureka: %v", err)
			return
		}
		for _, a := range apps {
			for _, ins := range a.Instances {
				url := fmt.Sprintf("http://%s:%d", ins.IPAddr, ins.Port)
				resp, err := httpClient.Get(url)
				if err != nil {
					log.Printf("❌ %s/%s inaccesible (timeout %s): %v", a.Name, ins.HostName, httpClient.Timeout, err)
				} else {
					resp.Body.Close()
					log.Printf("✅ %s/%s reachable (status %d)", a.Name, ins.HostName, resp.StatusCode)
				}
			}
		}
	})

	// Otro cron de ejemplo
	app.Cron().MustAdd("hello", "*/2 * * * *", func() {
		log.Println("Hello!")
	})

	// Arranque de PocketBase
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
