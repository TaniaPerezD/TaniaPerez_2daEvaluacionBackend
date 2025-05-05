# mega-sistema-backend-pocketbase
 

Esta extensión para **PocketBase**, escrita en **Go**, funciona como una capa intermedia de autenticación y autorización entre PocketBase y un microservicio de Zoom. Antes de acceder a los endpoints del microservicio, se verifica si el usuario posee los roles adecuados definidos en PocketBase.

---

## Funcionalidad

- Verifica tokens JWT emitidos por PocketBase.  
- Evalúa los roles del usuario antes de permitir el acceso a la API de Zoom.  
- Redirige o reenvía la solicitud al microservicio Spring si el rol es válido.  
- Rechaza peticiones no autorizadas con respuestas HTTP estándar.  
  Proporciona seguridad adicional sin modificar el backend principal.

---

## Tecnologías

- Go 1.21+  
- PocketBase 0.20+  
- net/http  
- JSON Web Tokens (JWT)  
- Custom PocketBase Extensions  
- Role-based access control

