package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"e-book/catalogo"
	"e-book/modelo"
	"e-book/usuario"
)

type RespuestaJSON struct {
	OK      bool        `json:"ok"`
	Mensaje string      `json:"mensaje,omitempty"`
	Datos   interface{} `json:"datos,omitempty"`
}

var (
	servicioCat        *catalogo.ServicioCatalogo
	usuariosBase       = make(map[string]*usuario.Usuario) // Mapeo por Email -> Usuario
	siguienteIDLibro   = 5
	siguienteIDUsuario = 102
)

func main() {
	// 1. Libros iniciales
	l1, _ := modelo.NewLibro(1, "Don Quijote de la Mancha", "Miguel de Cervantes", "Clásico", 863)
	l2, _ := modelo.NewLibro(2, "Cien Años de Soledad", "Gabriel García Márquez", "Realismo Mágico", 471)
	l3, _ := modelo.NewLibro(3, "El Principito", "Antoine de Saint-Exupéry", "Fábula", 96)
	l4, _ := modelo.NewLibro(4, "Fahrenheit 451", "Ray Bradbury", "Ciencia Ficción", 249)

	servicioCat = catalogo.NewServicioCatalogo([]*modelo.Libro{l1, l2, l3, l4})

	// 2. Usuarios base (Admin por defecto + Cliente de prueba)
	admin, _ := usuario.NewUsuario(100, "Administrador Principal", "admin@ebooks.com", "admin123", usuario.RolAdmin)
	clientePrueba, _ := usuario.NewUsuario(101, "Carlos Pérez", "carlos@gmail.com", "123456", usuario.RolCliente)

	usuariosBase[admin.Email] = admin
	usuariosBase[clientePrueba.Email] = clientePrueba

	// 3. Servidor HTTP corriendo en segundo plano
	go iniciarServidorHTTP()

	// 4. Iniciar flujo de consola (Login / Registro)
	flujoInicioSesion()
}

// ==========================================
// FLUJO DE AUTENTICACIÓN Y MENÚS
// ==========================================

func flujoInicioSesion() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n==================================================")
		fmt.Println("         📚 PLATAFORMA DIGITAL E-BOOK             ")
		fmt.Println("==================================================")
		fmt.Println("1. 🔑 Iniciar Sesión")
		fmt.Println("2. 📝 Registrarse como nuevo cliente")
		fmt.Println("3. 🚪 Salir")
		fmt.Println("--------------------------------------------------")
		fmt.Print("Seleccione una opción: ")

		opcion, _ := reader.ReadString('\n')
		opcion = strings.TrimSpace(opcion)

		switch opcion {
		case "1":
			fmt.Println("\n--- 🔑 INICIO DE SESIÓN ---")
			fmt.Print("Correo electrónico: ")
			email, _ := reader.ReadString('\n')
			email = strings.TrimSpace(email)

			fmt.Print("Contraseña: ")
			pass, _ := reader.ReadString('\n')
			pass = strings.TrimSpace(pass)

			usr, existe := usuariosBase[email]
			if !existe || !usr.ValidarPassword(pass) {
				fmt.Println("❌ Credenciales incorrectas. Verifique su correo o contraseña.")
				continue
			}

			fmt.Printf("\n✅ ¡Bienvenido/a %s! (Rol: %s)\n", usr.Nombre, usr.Rol)

			// REDIRECCIÓN SEGÚN ROL
			if usr.Rol == usuario.RolAdmin {
				menuAdministrador(usr, reader)
			} else {
				menuCliente(usr, reader)
			}

		case "2":
			fmt.Println("\n--- 📝 REGISTRO DE NUEVO CLIENTE ---")
			fmt.Print("Nombre completo: ")
			nombre, _ := reader.ReadString('\n')
			fmt.Print("Correo electrónico: ")
			email, _ := reader.ReadString('\n')
			fmt.Print("Contraseña: ")
			pass, _ := reader.ReadString('\n')

			email = strings.TrimSpace(email)
			if _, existe := usuariosBase[email]; existe {
				fmt.Println("❌ Ya existe un usuario registrado con ese correo.")
				continue
			}

			nuevoCliente, err := usuario.NewUsuario(siguienteIDUsuario, strings.TrimSpace(nombre), email, strings.TrimSpace(pass), usuario.RolCliente)
			if err != nil {
				fmt.Printf("❌ Error al registrarse: %v\n", err)
				continue
			}

			usuariosBase[email] = nuevoCliente
			siguienteIDUsuario++
			fmt.Println("🎉 ¡Registro exitoso! Ya puede iniciar sesión con sus credenciales.")

		case "3":
			fmt.Println("\n👋 ¡Hasta luego!")
			os.Exit(0)

		default:
			fmt.Println("❌ Opción no válida.")
		}
	}
}

// --- MENÚ CON PERMISOS DE ADMINISTRADOR ---
func menuAdministrador(admin *usuario.Usuario, reader *bufio.Reader) {
	for {
		fmt.Println("\n--------------------------------------------------")
		fmt.Printf("🛠️  PANEL DE ADMINISTRACIÓN | %s\n", admin.Nombre)
		fmt.Println("--------------------------------------------------")
		fmt.Println("1. ➕ Agregar nuevo libro al catálogo")
		fmt.Println("2. 📖 Ver catálogo completo")
		fmt.Println("3. 👥 Ver lista de todos los usuarios registrados")
		fmt.Println("4. 🔒 Cerrar sesión (Volver al inicio)")

		fmt.Print("Seleccione una opción: ")
		opcion, _ := reader.ReadString('\n')
		opcion = strings.TrimSpace(opcion)

		switch opcion {
		case "1":
			fmt.Println("\n--- ➕ AGREGAR LIBRO AL SISTEMA ---")
			fmt.Print("Título: ")
			titulo, _ := reader.ReadString('\n')
			fmt.Print("Autor: ")
			autor, _ := reader.ReadString('\n')
			fmt.Print("Género: ")
			genero, _ := reader.ReadString('\n')
			fmt.Print("Páginas Totales: ")
			paginasStr, _ := reader.ReadString('\n')

			paginas, err := strconv.Atoi(strings.TrimSpace(paginasStr))
			if err != nil || paginas <= 0 {
				fmt.Println("❌ Error: Número de páginas inválido.")
				continue
			}

			nuevoLibro, err := modelo.NewLibro(siguienteIDLibro, strings.TrimSpace(titulo), strings.TrimSpace(autor), strings.TrimSpace(genero), paginas)
			if err != nil {
				fmt.Printf("❌ Error al crear el libro: %v\n", err)
				continue
			}

			libros := append(servicioCat.ObtenerTodos(), nuevoLibro)
			servicioCat = catalogo.NewServicioCatalogo(libros)
			fmt.Printf("✅ Libro '%s' publicado en la plataforma con ID: %d!\n", nuevoLibro.Titulo, siguienteIDLibro)
			siguienteIDLibro++

		case "2":
			fmt.Println("\n--- 📖 CATÁLOGO DEL SISTEMA ---")
			for _, l := range servicioCat.ObtenerTodos() {
				fmt.Printf("ID: %d | Título: %s | Autor: %s | Género: %s | Páginas: %d\n",
					l.ID, l.Titulo, l.Autor, l.Genero, l.PaginasTotales)
			}

		case "3":
			fmt.Println("\n--- 👥 USUARIOS EN LA PLATAFORMA ---")
			for _, u := range usuariosBase {
				fmt.Printf("ID: %d | Nombre: %s | Email: %s | Rol: %s\n", u.ID, u.Nombre, u.Email, u.Rol)
			}

		case "4":
			fmt.Println("🔒 Sesión de administrador cerrada.")
			return // Sale del menú de admin y regresa al flujo principal

		default:
			fmt.Println("❌ Opción inválida.")
		}
	}
}

// --- MENÚ CON PERMISOS DE CLIENTE ---
func menuCliente(cliente *usuario.Usuario, reader *bufio.Reader) {
	for {
		fmt.Println("\n--------------------------------------------------")
		fmt.Printf("📖 PANEL DE LECTOR / CLIENTE | %s\n", cliente.Nombre)
		fmt.Println("--------------------------------------------------")
		fmt.Println("1. 📚 Consultar catálogo de libros")
		fmt.Println("2. ⭐ Agregar libro a mis favoritos")
		fmt.Println("3. 🔖 Ver mis libros favoritos")
		fmt.Println("4. 🔒 Cerrar sesión")

		fmt.Print("Seleccione una opción: ")
		opcion, _ := reader.ReadString('\n')
		opcion = strings.TrimSpace(opcion)

		switch opcion {
		case "1":
			fmt.Println("\n--- 📚 CATÁLOGO DE LIBROS DISPONIBLES ---")
			for _, l := range servicioCat.ObtenerTodos() {
				fmt.Printf("ID: %d | Título: %s | Autor: %s | Género: %s\n", l.ID, l.Titulo, l.Autor, l.Genero)
			}

		case "2":
			fmt.Print("Ingrese el ID del libro que desea guardar en favoritos: ")
			idStr, _ := reader.ReadString('\n')
			id, err := strconv.Atoi(strings.TrimSpace(idStr))
			if err != nil {
				fmt.Println("❌ ID inválido.")
				continue
			}

			var libroEncontrado *modelo.Libro
			for _, l := range servicioCat.ObtenerTodos() {
				if l.ID == id {
					libroEncontrado = l
					break
				}
			}

			if libroEncontrado == nil {
				fmt.Println("❌ Libro no encontrado en el catálogo.")
				continue
			}

			if err := cliente.AgregarFavorito(libroEncontrado); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Printf("⭐ Libro '%s' guardado en tus favoritos.\n", libroEncontrado.Titulo)
			}

		case "3":
			fmt.Println("\n--- ⭐ MIS LIBROS FAVORITOS ---")
			favs := cliente.ObtenerFavoritos()
			if len(favs) == 0 {
				fmt.Println("Aún no tienes libros agregados a favoritos.")
			} else {
				for _, l := range favs {
					fmt.Printf("- %s (Autor: %s)\n", l.Titulo, l.Autor)
				}
			}

		case "4":
			fmt.Println("🔒 Sesión cerrada.")
			return

		default:
			fmt.Println("❌ Opción inválida.")
		}
	}
}

// ==========================================
// SERVIDOR WEB HTTP
// ==========================================

func iniciarServidorHTTP() {
	http.HandleFunc("/api/libros", func(w http.ResponseWriter, r *http.Request) {
		enviarJSON(w, http.StatusOK, RespuestaJSON{OK: true, Datos: servicioCat.ObtenerTodos()})
	})

	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		enviarJSON(w, http.StatusOK, RespuestaJSON{OK: true, Mensaje: "Servidor activo"})
	})

	_ = http.ListenAndServe(":8080", nil)
}

func enviarJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
