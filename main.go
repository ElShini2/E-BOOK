package main

import (
	"bufio"
	"e-book/catalogo"
	"e-book/modelo"
	"e-book/usuario"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const archivoUsuarios = "usuarios.json"

var (
	servicioCat        *catalogo.ServicioCatalogo
	usuariosBase       map[string]*usuario.Usuario
	usuarioActivo      *usuario.Usuario
	siguienteIDUsuario = 103
	mu                 sync.RWMutex
)

// Guardar usuarios en disco en formato JSON legible
func guardarUsuariosEnDisco() {
	mu.RLock()
	defer mu.RUnlock()

	var lista []*usuario.Usuario
	for _, u := range usuariosBase {
		lista = append(lista, u)
	}

	datos, err := json.MarshalIndent(lista, "", "  ")
	if err == nil {
		_ = os.WriteFile(archivoUsuarios, datos, 0644)
	}
}

// Cargar usuarios desde el archivo JSON si existe
func cargarUsuariosDesdeDisco() {
	if _, err := os.Stat(archivoUsuarios); os.IsNotExist(err) {
		return
	}

	datos, err := os.ReadFile(archivoUsuarios)
	if err != nil {
		return
	}

	var lista []*usuario.Usuario
	if err := json.Unmarshal(datos, &lista); err == nil {
		for _, u := range lista {
			usuariosBase[u.Email] = u
			if u.ID >= siguienteIDUsuario {
				siguienteIDUsuario = u.ID + 1
			}
		}
	}
}

func init() {
	usuariosBase = make(map[string]*usuario.Usuario)
	servicioCat = catalogo.NewServicioCatalogo()

	// 1. Cargar usuarios existentes desde el archivo
	cargarUsuariosDesdeDisco()

	// 2. Si no hay usuarios en disco, inicializar cuentas predeterminadas
	if len(usuariosBase) == 0 {
		admin, _ := usuario.NewUsuario(101, "Administrador Root", "admin@ebooks.com", "admin123", usuario.RolAdmin)
		cliente, _ := usuario.NewUsuario(102, "Michaell Salvador", "cliente@ebooks.com", "cliente123", usuario.RolCliente)

		usuariosBase[admin.Email] = admin
		usuariosBase[cliente.Email] = cliente
		guardarUsuariosEnDisco()
	}

	// 3. Inicializar catálogo base si el archivo está vacío
	if len(servicioCat.ObtenerTodos()) == 0 {
		l1, _ := modelo.NewLibro(1, "El Quijote", "Miguel de Cervantes", "Clasico", 863)
		l2, _ := modelo.NewLibro(2, "Cien Anos de Soledad", "Gabriel Garcia Marquez", "Realismo Magico", 471)
		l3, _ := modelo.NewLibro(3, "1984", "George Orwell", "Ciencia Ficcion", 328)

		_ = servicioCat.AgregarLibro(l1)
		_ = servicioCat.AgregarLibro(l2)
		_ = servicioCat.AgregarLibro(l3)
	}

	// Inicia sin sesión activa para obligar a pasar por el Login
	usuarioActivo = nil
}

// =============================================================
// SERVICIOS WEB HTTP (8 ENDPOINTS CON SERIALIZACIÓN JSON)
// =============================================================

// 1. GET /api/status -> Comprueba el estado del backend
func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	respuesta := map[string]interface{}{
		"estado":      "Operativo",
		"modulo":      "E-Book Backend Server",
		"timestamp":   time.Now().Format(time.RFC3339),
		"codigo_http": http.StatusOK,
	}
	_ = json.NewEncoder(w).Encode(respuesta)
}

// 2. GET /api/libros -> Listado completo del catálogo
func handleListarLibros(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	libros := servicioCat.ObtenerTodos()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":  len(libros),
		"libros": libros,
	})
}

// 3. GET /api/libros/detalle?id=1 -> Ficha técnica de un libro por su ID
func handleDetalleLibro(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, `{"error": "Debe proporcionar el parámetro 'id'"}`, http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error": "El ID debe ser numérico"}`, http.StatusBadRequest)
		return
	}

	for _, l := range servicioCat.ObtenerTodos() {
		if l.ID == id {
			_ = json.NewEncoder(w).Encode(l)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "Libro no encontrado en el catálogo"})
}

// 4. GET /api/libros/buscar?titulo=... -> Búsqueda por coincidencia de texto
func handleBuscarLibro(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	titulo := r.URL.Query().Get("titulo")
	if titulo == "" {
		http.Error(w, `{"error": "Debe proporcionar el parámetro 'titulo'"}`, http.StatusBadRequest)
		return
	}

	libro, err := servicioCat.BuscarPorTitulo(titulo)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(libro)
}

// 5. GET /api/usuarios -> Listado general de usuarios (Uso administrativo)
func handleListarUsuarios(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mu.RLock()
	defer mu.RUnlock()

	var lista []*usuario.Usuario
	for _, u := range usuariosBase {
		lista = append(lista, u)
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total_usuarios": len(lista),
		"usuarios":       lista,
	})
}

// 6. GET /api/usuario/perfil -> Información de la sesión activa
func handlePerfilUsuario(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mu.RLock()
	defer mu.RUnlock()

	if usuarioActivo == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "No hay un usuario autenticado en la sesión de consola",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(usuarioActivo)
}

// 7. GET /api/usuario/favoritos -> Libros favoritos de la sesión activa
func handleFavoritos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mu.RLock()
	defer mu.RUnlock()

	if usuarioActivo == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "No hay un usuario autenticado en la sesión de consola",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"usuario":   usuarioActivo.Nombre,
		"favoritos": usuarioActivo.ObtenerFavoritos(),
	})
}

// 8. GET /api/estadisticas -> Métricas globales del sistema
func handleEstadisticas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mu.RLock()
	defer mu.RUnlock()

	libros := servicioCat.ObtenerTodos()
	totalPaginas := 0
	for _, l := range libros {
		totalPaginas += l.PaginasTotales
	}

	sesion := "Ninguna (Sesión no iniciada)"
	if usuarioActivo != nil {
		sesion = fmt.Sprintf("%s (%s)", usuarioActivo.Nombre, usuarioActivo.Rol)
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total_libros_disponibles": len(libros),
		"total_usuarios_sistema":   len(usuariosBase),
		"total_paginas_catalogo":   totalPaginas,
		"sesion_activa":            sesion,
		"persistencia":             "Sincronización JSON Activa",
	})
}

// Endpoint Protegido: Eliminar libros (Solo ADMIN)
func handleEliminarLibro(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, `{"error": "Método no permitido. Use DELETE o POST"}`, http.StatusMethodNotAllowed)
		return
	}

	mu.RLock()
	if usuarioActivo == nil || usuarioActivo.Rol != usuario.RolAdmin {
		mu.RUnlock()
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Acceso denegado: Solo un usuario con rol ADMIN puede eliminar existencias",
		})
		return
	}
	mu.RUnlock()

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, `{"error": "Debe proporcionar el parámetro 'id'"}`, http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error": "El ID debe ser un número entero"}`, http.StatusBadRequest)
		return
	}

	err = servicioCat.EliminarLibroPorID(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":       "Libro eliminado exitosamente del catálogo y persistido en disco",
		"id_eliminado":  id,
		"ejecutado_por": usuarioActivo.Nombre,
	})
}

func iniciarServidorHTTP() {
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/libros", handleListarLibros)
	http.HandleFunc("/api/libros/detalle", handleDetalleLibro)
	http.HandleFunc("/api/libros/buscar", handleBuscarLibro)
	http.HandleFunc("/api/usuarios", handleListarUsuarios)
	http.HandleFunc("/api/usuario/perfil", handlePerfilUsuario)
	http.HandleFunc("/api/usuario/favoritos", handleFavoritos)
	http.HandleFunc("/api/estadisticas", handleEstadisticas)
	http.HandleFunc("/api/libros/eliminar", handleEliminarLibro)

	_ = http.ListenAndServe(":8080", nil)
}

// =============================================================
// INTERFAZ DE COMANDOS DE CONSOLA (CLI)
// =============================================================

func main() {
	go iniciarServidorHTTP()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("====================================================")
	fmt.Println("       SISTEMA DE GESTION DE E-BOOKS EN GO          ")
	fmt.Println("  Servidor Web Activo: http://localhost:8080/api/   ")
	fmt.Println("  Persistencia: Archivos JSON activos en disco      ")
	fmt.Println("====================================================")

	for {
		if usuarioActivo == nil {
			flujoInicioSesion(scanner)
		} else {
			if usuarioActivo.Rol == usuario.RolAdmin {
				menuAdministrador(scanner)
			} else {
				menuCliente(scanner)
			}
		}
	}
}

func flujoInicioSesion(scanner *bufio.Scanner) {
	fmt.Println("\n--- MENÚ PRINCIPAL: ACCESO AL SISTEMA ---")
	fmt.Println("1. Iniciar Sesión")
	fmt.Println("2. Registrar Nuevo Cliente")
	fmt.Println("3. Salir de la Aplicación")
	fmt.Print("Seleccione una opción (1-3): ")

	scanner.Scan()
	opcion := strings.TrimSpace(scanner.Text())

	switch opcion {
	case "1":
		fmt.Print("Correo electrónico: ")
		scanner.Scan()
		email := strings.ToLower(strings.TrimSpace(scanner.Text()))

		fmt.Print("Contraseña: ")
		scanner.Scan()
		pass := strings.TrimSpace(scanner.Text())

		mu.RLock()
		u, existe := usuariosBase[email]
		mu.RUnlock()

		if !existe || !u.ValidarPassword(pass) {
			fmt.Println("❌ Credenciales incorrectas. Verifique correo y contraseña.")
			return
		}

		mu.Lock()
		usuarioActivo = u
		mu.Unlock()
		fmt.Printf("\n✅ Autenticación exitosa. Bienvenido %s (Rol: %s)\n", u.Nombre, u.Rol)

	case "2":
		fmt.Print("Nombre completo: ")
		scanner.Scan()
		nombre := strings.TrimSpace(scanner.Text())

		fmt.Print("Correo electrónico: ")
		scanner.Scan()
		email := strings.ToLower(strings.TrimSpace(scanner.Text()))

		fmt.Print("Contraseña: ")
		scanner.Scan()
		pass := strings.TrimSpace(scanner.Text())

		mu.Lock()
		if _, existe := usuariosBase[email]; existe {
			mu.Unlock()
			fmt.Println("❌ Error: Ya existe un usuario registrado con ese correo.")
			return
		}

		nuevo, err := usuario.NewUsuario(siguienteIDUsuario, nombre, email, pass, usuario.RolCliente)
		if err != nil {
			mu.Unlock()
			fmt.Printf("❌ Error al crear usuario: %v\n", err)
			return
		}

		usuariosBase[email] = nuevo
		siguienteIDUsuario++
		usuarioActivo = nuevo
		mu.Unlock()

		guardarUsuariosEnDisco()
		fmt.Printf("\n✅ Registro exitoso. Sesión iniciada automáticamente como %s.\n", nuevo.Nombre)

	case "3":
		fmt.Println("Cerrando la aplicación...")
		os.Exit(0)
	default:
		fmt.Println("Opción no válida.")
	}
}

func menuAdministrador(scanner *bufio.Scanner) {
	fmt.Printf("\n=== PANEL DE CONTROL (ADMIN: %s) ===\n", usuarioActivo.Nombre)
	fmt.Println("1. Ver Catálogo Completo")
	fmt.Println("2. Agregar Nuevo Libro")
	fmt.Println("3. Eliminar Libro del Catálogo")
	fmt.Println("4. Listar Usuarios Registrados")
	fmt.Println("5. Ver Resumen Estadístico")
	fmt.Println("6. Cerrar Sesión")
	fmt.Print("Seleccione una opción: ")

	scanner.Scan()
	opcion := strings.TrimSpace(scanner.Text())

	switch opcion {
	case "1":
		listarLibrosConsola()
	case "2":
		agregarLibroConsola(scanner)
	case "3":
		eliminarLibroConsola(scanner)
	case "4":
		mu.RLock()
		fmt.Println("\n--- USUARIOS REGISTRADOS ---")
		for _, u := range usuariosBase {
			fmt.Printf("- [ID: %d] %s (%s) | Rol: %s\n", u.ID, u.Nombre, u.Email, u.Rol)
		}
		mu.RUnlock()
	case "5":
		mu.RLock()
		fmt.Printf("\n--- ESTADÍSTICAS DEL SISTEMA ---\n")
		fmt.Printf("Total E-books: %d\n", len(servicioCat.ObtenerTodos()))
		fmt.Printf("Total Usuarios: %d\n", len(usuariosBase))
		mu.RUnlock()
	case "6":
		mu.Lock()
		usuarioActivo = nil
		mu.Unlock()
		fmt.Println("✅ Sesión cerrada correctamente.")
	default:
		fmt.Println("Opción no válida.")
	}
}

func menuCliente(scanner *bufio.Scanner) {
	fmt.Printf("\n=== BIBLIOTECA DIGITAL (CLIENTE: %s) ===\n", usuarioActivo.Nombre)
	fmt.Println("1. Ver Catálogo de E-Books")
	fmt.Println("2. Buscar Libro por Título")
	fmt.Println("3. Agregar Libro a Mis Favoritos")
	fmt.Println("4. Ver Mis Favoritos")
	fmt.Println("5. Cerrar Sesión")
	fmt.Print("Seleccione una opción: ")

	scanner.Scan()
	opcion := strings.TrimSpace(scanner.Text())

	switch opcion {
	case "1":
		listarLibrosConsola()
	case "2":
		fmt.Print("Ingrese el título o fragmento: ")
		scanner.Scan()
		busqueda := strings.TrimSpace(scanner.Text())

		l, err := servicioCat.BuscarPorTitulo(busqueda)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("📖 Encontrado: [ID: %d] '%s' - %s (%d págs.)\n", l.ID, l.Titulo, l.Autor, l.PaginasTotales)
		}

	case "3":
		listarLibrosConsola()
		fmt.Print("Ingrese el ID del libro a agregar: ")
		scanner.Scan()
		id, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil {
			fmt.Println("❌ ID inválido.")
			return
		}

		var libroSeleccionado *modelo.Libro
		for _, l := range servicioCat.ObtenerTodos() {
			if l.ID == id {
				libroSeleccionado = l
				break
			}
		}

		if libroSeleccionado == nil {
			fmt.Println("❌ No se encontró ningún libro con ese ID.")
			return
		}

		mu.Lock()
		err = usuarioActivo.AgregarFavorito(libroSeleccionado)
		mu.Unlock()

		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			guardarUsuariosEnDisco()
			fmt.Printf("⭐ '%s' añadido a tus favoritos.\n", libroSeleccionado.Titulo)
		}

	case "4":
		mu.RLock()
		favs := usuarioActivo.ObtenerFavoritos()
		fmt.Println("\n--- MIS LIBROS FAVORITOS ---")
		if len(favs) == 0 {
			fmt.Println("(No tienes libros en favoritos)")
		} else {
			for _, l := range favs {
				fmt.Printf("⭐ [ID: %d] %s (%s)\n", l.ID, l.Titulo, l.Autor)
			}
		}
		mu.RUnlock()

	case "5":
		mu.Lock()
		usuarioActivo = nil
		mu.Unlock()
		fmt.Println("✅ Sesión cerrada correctamente.")
	default:
		fmt.Println("Opción no válida.")
	}
}

func listarLibrosConsola() {
	fmt.Println("\n--- CATÁLOGO DE E-BOOKS ---")
	libros := servicioCat.ObtenerTodos()
	if len(libros) == 0 {
		fmt.Println("(El catálogo está vacío)")
		return
	}
	for _, l := range libros {
		fmt.Printf("[%d] %s - %s | Género: %s | Págs: %d\n", l.ID, l.Titulo, l.Autor, l.Genero, l.PaginasTotales)
	}
}

func agregarLibroConsola(scanner *bufio.Scanner) {
	fmt.Println("\n--- REGISTRO DE NUEVO LIBRO ---")

	// Asignación automática del ID
	nuevoID := servicioCat.ObtenerSiguienteID()
	fmt.Printf("ID asignado automáticamente: %d\n", nuevoID)

	fmt.Print("Título: ")
	scanner.Scan()
	titulo := strings.TrimSpace(scanner.Text())
	if titulo == "" {
		fmt.Println("❌ El título no puede estar vacío.")
		return
	}

	fmt.Print("Autor: ")
	scanner.Scan()
	autor := strings.TrimSpace(scanner.Text())
	if autor == "" {
		fmt.Println("❌ El autor no puede estar vacío.")
		return
	}

	fmt.Print("Género: ")
	scanner.Scan()
	genero := strings.TrimSpace(scanner.Text())
	if genero == "" {
		fmt.Println("❌ El género no puede estar vacío.")
		return
	}

	fmt.Print("Páginas Totales: ")
	scanner.Scan()
	paginas, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || paginas <= 0 {
		fmt.Println("❌ Número de páginas inválido (debe ser mayor a 0).")
		return
	}

	nuevoLibro, err := modelo.NewLibro(nuevoID, titulo, autor, genero, paginas)
	if err != nil {
		fmt.Printf("❌ Error de validación: %v\n", err)
		return
	}

	err = servicioCat.AgregarLibro(nuevoLibro)
	if err != nil {
		fmt.Printf("❌ Error al guardar: %v\n", err)
		return
	}

	fmt.Printf("✅ Libro '%s' registrado con ID %d y guardado en disco exitosamente.\n", titulo, nuevoID)
}

func eliminarLibroConsola(scanner *bufio.Scanner) {
	mu.RLock()
	if usuarioActivo == nil || usuarioActivo.Rol != usuario.RolAdmin {
		mu.RUnlock()
		fmt.Println("❌ Error de permisos: Solo un Administrador puede eliminar libros.")
		return
	}
	mu.RUnlock()

	listarLibrosConsola()
	fmt.Print("\nIngrese el ID del libro que desea eliminar (o escriba '0' para cancelar): ")
	scanner.Scan()
	entrada := strings.TrimSpace(scanner.Text())

	// Escape seguro
	if entrada == "0" || entrada == "" {
		fmt.Println("↩️ Operación cancelada. No se eliminó ningún libro.")
		return
	}

	id, err := strconv.Atoi(entrada)
	if err != nil {
		fmt.Println("❌ ID inválido. La operación ha sido cancelada.")
		return
	}

	err = servicioCat.EliminarLibroPorID(id)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("🗑️ Libro con ID %d eliminado y actualizado en disco.\n", id)
	}
}
