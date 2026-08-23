package main

import (
	"bufio"
	"e-book/catalogo"
	"e-book/modelo"
	"e-book/usuario"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	archivoUsuarios = "usuarios.json"
	carpetaPDFs     = "uploads"
)

var (
	servicioCat        *catalogo.ServicioCatalogo
	usuariosBase       map[string]*usuario.Usuario
	usuarioActivo      *usuario.Usuario
	siguienteIDUsuario = 103
	mu                 sync.RWMutex
)

func abrirNavegador(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		fmt.Printf("⚠️ Abre manualmente este enlace: %s\n", url)
	}
}

func contarPaginasPDF(ruta string) (int, error) {
	contenido, err := os.ReadFile(ruta)
	if err != nil {
		return 0, err
	}

	reCount := regexp.MustCompile(`/Type\s*/Pages.*?/Count\s+(\d+)`)
	coincidencias := reCount.FindSubmatch(contenido)
	if len(coincidencias) > 1 {
		if paginas, err := strconv.Atoi(string(coincidencias[1])); err == nil && paginas > 0 {
			return paginas, nil
		}
	}

	rePage := regexp.MustCompile(`/Type\s*/Page\b`)
	paginas := len(rePage.FindAll(contenido, -1))
	if paginas > 0 {
		return paginas, nil
	}

	return 1, nil
}

func guardarUsuariosEnDisco() {
	var lista []*usuario.Usuario
	for _, u := range usuariosBase {
		lista = append(lista, u)
	}

	datos, err := json.MarshalIndent(lista, "", "  ")
	if err == nil {
		_ = os.WriteFile(archivoUsuarios, datos, 0644)
	}
}

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
			if u.ProgresoLectura == nil {
				u.ProgresoLectura = make(map[string]int)
			}
			usuariosBase[u.Email] = u
			if u.ID >= siguienteIDUsuario {
				siguienteIDUsuario = u.ID + 1
			}
		}
	}
}

func init() {
	_ = os.MkdirAll(carpetaPDFs, 0755)
	usuariosBase = make(map[string]*usuario.Usuario)
	servicioCat = catalogo.NewServicioCatalogo()

	cargarUsuariosDesdeDisco()

	if len(usuariosBase) == 0 {
		admin, _ := usuario.NewUsuario(101, "Administrador Root", "admin@ebooks.com", "admin123", usuario.RolAdmin)
		cliente, _ := usuario.NewUsuario(102, "Michaell Salvador", "cliente@ebooks.com", "cliente123", usuario.RolCliente)

		usuariosBase[admin.Email] = admin
		usuariosBase[cliente.Email] = cliente
		guardarUsuariosEnDisco()
	}

	usuarioActivo = nil
}

// =============================================================
// SERVICIOS WEB HTTP Y VISOR DE PÁGINA ÚNICA
// =============================================================

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"estado":    "Operativo",
		"modulo":    "E-Book Backend Server con Visor E-Reader",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func handleListarLibros(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	libros := servicioCat.ObtenerTodos()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":  len(libros),
		"libros": libros,
	})
}

// Endpoint para guardar progreso automáticamente en segundo plano
func handleGuardarProgreso(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Método no permitido"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email   string `json:"email"`
		LibroID int    `json:"libro_id"`
		Pagina  int    `json:"pagina"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "JSON inválido"}`, http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	u, existe := usuariosBase[strings.ToLower(strings.TrimSpace(req.Email))]
	if !existe {
		if usuarioActivo != nil {
			u = usuarioActivo
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Usuario no identificado"})
			return
		}
	}

	u.ActualizarProgreso(req.LibroID, req.Pagina)
	guardarUsuariosEnDisco()

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":            true,
		"libro_id":      req.LibroID,
		"pagina_actual": req.Pagina,
	})
}

// Lector con renderizado de página única y guardado automático
func handleVisorPDF(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var libroSel *modelo.Libro
	for _, l := range servicioCat.ObtenerTodos() {
		if l.ID == id {
			libroSel = l
			break
		}
	}

	if libroSel == nil {
		http.Error(w, "Libro no encontrado", http.StatusNotFound)
		return
	}

	userEmail := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("user")))

	mu.RLock()
	var u *usuario.Usuario
	if userEmail != "" {
		u = usuariosBase[userEmail]
	}
	if u == nil && usuarioActivo != nil {
		u = usuarioActivo
	}
	mu.RUnlock()

	ultimaPagina := 1
	nombreUsuario := "Invitado"
	emailFinal := ""

	if u != nil {
		ultimaPagina = u.ObtenerUltimaPagina(libroSel.ID)
		nombreUsuario = u.Nombre
		emailFinal = u.Email
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <title>E-Reader: %s</title>
    <!-- Motor PDF.js de Mozilla -->
    <script src="https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.min.js"></script>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: 'Segoe UI', system-ui, sans-serif; background: #14141d; color: #f0f0f5; display: flex; flex-direction: column; height: 100vh; overflow: hidden; }
        .toolbar { background: #1f1f2e; padding: 12px 24px; display: flex; justify-content: space-between; align-items: center; border-bottom: 2px solid #2e2e42; z-index: 10; box-shadow: 0 4px 15px rgba(0,0,0,0.4); }
        .book-title { font-size: 16px; font-weight: 600; color: #fff; }
        .book-title span { color: #00d2ff; }
        .controls { display: flex; align-items: center; gap: 10px; }
        button { background: #2b2b40; color: #fff; border: 1px solid #41415e; padding: 7px 15px; border-radius: 6px; cursor: pointer; font-size: 14px; font-weight: 600; transition: all 0.2s; display: flex; align-items: center; gap: 5px; }
        button:hover { background: #00d2ff; color: #000; border-color: #00d2ff; }
        button:disabled { opacity: 0.3; cursor: not-allowed; }
        .page-indicator { font-size: 14px; font-weight: bold; background: #0f0f17; padding: 6px 14px; border-radius: 6px; border: 1px solid #33334d; }
        .sync-badge { font-size: 12px; color: #00e676; opacity: 0; transition: opacity 0.3s; margin-left: 8px; font-weight: bold; }
        
        .viewport { flex: 1; display: flex; justify-content: center; align-items: center; overflow: auto; padding: 20px; background: #101018; }
        #canvas-container { position: relative; box-shadow: 0 10px 30px rgba(0,0,0,0.7); border-radius: 4px; overflow: hidden; background: #fff; line-height: 0; }
        canvas { display: block; max-height: 85vh; max-width: 90vw; object-fit: contain; }
        
        .loading-overlay { position: absolute; inset: 0; background: rgba(20,20,29,0.85); display: flex; justify-content: center; align-items: center; font-size: 18px; font-weight: bold; color: #00d2ff; z-index: 5; }
    </style>
</head>
<body>
    <div class="toolbar">
        <div class="book-title">
            📖 <span>%s</span> | Autor: %s | Lector: <i>%s</i>
        </div>
        <div class="controls">
            <button id="prevBtn" onclick="cambiarPagina(-1)">◀ Anterior</button>
            <div class="page-indicator">
                Pág <span id="currentPagNum">%d</span> / <span id="totalPagNum">%d</span>
            </div>
            <button id="nextBtn" onclick="cambiarPagina(1)">Siguiente ▶</button>
            <span id="syncStatus" class="sync-badge">✓ Guardado</span>
        </div>
    </div>

    <div class="viewport">
        <div id="canvas-container">
            <div id="loader" class="loading-overlay">Cargando libro...</div>
            <canvas id="pdfCanvas"></canvas>
        </div>
    </div>

    <script>
        pdfjsLib.GlobalWorkerOptions.workerSrc = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.worker.min.js';

        const urlPDF = '/archivos/%s';
        const libroID = %d;
        const userEmail = '%s';
        let pdfDoc = null;
        let paginaActual = %d;
        let totalPaginas = %d;
        let renderizando = false;
        let paginaPendiente = null;

        const canvas = document.getElementById('pdfCanvas');
        const ctx = canvas.getContext('2d');

        function renderizarPagina(num) {
            renderizando = true;
            document.getElementById('currentPagNum').textContent = num;

            pdfDoc.getPage(num).then(function(page) {
                const viewport = page.getViewport({ scale: 1.5 });
                canvas.height = viewport.height;
                canvas.width = viewport.width;

                const renderContext = {
                    canvasContext: ctx,
                    viewport: viewport
                };

                const renderTask = page.render(renderContext);
                renderTask.promise.then(function() {
                    renderizando = false;
                    document.getElementById('loader').style.display = 'none';

                    // Actualizar botones
                    document.getElementById('prevBtn').disabled = (num <= 1);
                    document.getElementById('nextBtn').disabled = (num >= totalPaginas);

                    if (paginaPendiente !== null) {
                        renderizarPagina(paginaPendiente);
                        paginaPendiente = null;
                    }
                });
            });
        }

        function encolarRenderizado(num) {
            if (renderizando) {
                paginaPendiente = num;
            } else {
                renderizarPagina(num);
            }
        }

        function autoguardarProgreso(pag) {
            const badge = document.getElementById('syncStatus');
            fetch('/api/progreso', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email: userEmail, libro_id: libroID, pagina: pag })
            })
            .then(res => res.json())
            .then(() => {
                badge.style.opacity = '1';
                setTimeout(() => { badge.style.opacity = '0'; }, 2000);
            })
            .catch(() => {});
        }

        function cambiarPagina(delta) {
            let nueva = paginaActual + delta;
            if (nueva < 1 || nueva > totalPaginas) return;

            paginaActual = nueva;
            encolarRenderizado(paginaActual);
            autoguardarProgreso(paginaActual); // Guardado automático inmediato
        }

        // Navegación con teclado (Flecha izquierda / derecha)
        window.addEventListener('keydown', function(e) {
            if (e.key === 'ArrowLeft') cambiarPagina(-1);
            if (e.key === 'ArrowRight') cambiarPagina(1);
        });

        // Inicializar documento
        pdfjsLib.getDocument(urlPDF).promise.then(function(pdf) {
            pdfDoc = pdf;
            totalPaginas = pdf.numPages;
            document.getElementById('totalPagNum').textContent = totalPaginas;

            if (paginaActual > totalPaginas) paginaActual = totalPaginas;
            if (paginaActual < 1) paginaActual = 1;

            renderizarPagina(paginaActual);
        }).catch(function(err) {
            document.getElementById('loader').textContent = 'Error al cargar el documento PDF.';
        });
    </script>
</body>
</html>`, libroSel.Titulo, libroSel.Titulo, libroSel.Autor, nombreUsuario, ultimaPagina, libroSel.PaginasTotales, libroSel.ArchivoPDF, libroSel.ID, emailFinal, ultimaPagina, libroSel.PaginasTotales)

	_, _ = w.Write([]byte(html))
}

func iniciarServidorHTTP() {
	fs := http.FileServer(http.Dir(carpetaPDFs))
	http.Handle("/archivos/", http.StripPrefix("/archivos/", fs))

	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/libros", handleListarLibros)
	http.HandleFunc("/api/progreso", handleGuardarProgreso)
	http.HandleFunc("/lector", handleVisorPDF)

	_ = http.ListenAndServe(":8080", nil)
}

// =============================================================
// CONSOLA CLI
// =============================================================

func main() {
	go iniciarServidorHTTP()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("====================================================")
	fmt.Println("   SISTEMA DE E-BOOKS (E-READER CON AUTO-GUARDADO)   ")
	fmt.Println("  Servidor Web Activo: http://localhost:8080        ")
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
	fmt.Println("\n--- MENÚ PRINCIPAL ---")
	fmt.Println("1. Iniciar Sesión")
	fmt.Println("2. Registrar Nuevo Cliente")
	fmt.Println("3. Salir")
	fmt.Print("Seleccione una opción: ")

	scanner.Scan()
	opcion := strings.TrimSpace(scanner.Text())

	switch opcion {
	case "1":
		fmt.Print("Correo: ")
		scanner.Scan()
		email := strings.ToLower(strings.TrimSpace(scanner.Text()))
		fmt.Print("Contraseña: ")
		scanner.Scan()
		pass := strings.TrimSpace(scanner.Text())

		mu.RLock()
		u, existe := usuariosBase[email]
		mu.RUnlock()

		if !existe || !u.ValidarPassword(pass) {
			fmt.Println("❌ Credenciales incorrectas.")
			return
		}

		mu.Lock()
		usuarioActivo = u
		mu.Unlock()
		fmt.Printf("\n✅ Bienvenido %s (%s)\n", u.Nombre, u.Rol)

	case "2":
		fmt.Print("Nombre completo: ")
		scanner.Scan()
		nombre := strings.TrimSpace(scanner.Text())
		fmt.Print("Correo: ")
		scanner.Scan()
		email := strings.ToLower(strings.TrimSpace(scanner.Text()))
		fmt.Print("Contraseña: ")
		scanner.Scan()
		pass := strings.TrimSpace(scanner.Text())

		mu.Lock()
		if _, existe := usuariosBase[email]; existe {
			mu.Unlock()
			fmt.Println("❌ Correo ya registrado.")
			return
		}

		nuevo, err := usuario.NewUsuario(siguienteIDUsuario, nombre, email, pass, usuario.RolCliente)
		if err != nil {
			mu.Unlock()
			fmt.Printf("❌ Error: %v\n", err)
			return
		}

		usuariosBase[email] = nuevo
		siguienteIDUsuario++
		usuarioActivo = nuevo
		guardarUsuariosEnDisco()
		mu.Unlock()

		fmt.Printf("\n✅ Usuario registrado. Sesión activa: %s\n", nuevo.Nombre)

	case "3":
		os.Exit(0)
	}
}

func menuAdministrador(scanner *bufio.Scanner) {
	fmt.Printf("\n=== PANEL ADMINISTRADOR (%s) ===\n", usuarioActivo.Nombre)
	fmt.Println("1. Ver Catálogo")
	fmt.Println("2. Cargar Libro (.PDF)")
	fmt.Println("3. Eliminar Libro")
	fmt.Println("4. Cerrar Sesión")
	fmt.Print("Opción: ")

	scanner.Scan()
	switch strings.TrimSpace(scanner.Text()) {
	case "1":
		listarLibrosConsola()
	case "2":
		cargarLibroPDFConsola(scanner)
	case "3":
		eliminarLibroConsola(scanner)
	case "4":
		mu.Lock()
		usuarioActivo = nil
		mu.Unlock()
	}
}

func menuCliente(scanner *bufio.Scanner) {
	fmt.Printf("\n=== BIBLIOTECA DIGITAL (%s) ===\n", usuarioActivo.Nombre)
	fmt.Println("1. Ver Catálogo")
	fmt.Println("2. Leer Libro (E-Reader)")
	fmt.Println("3. Cerrar Sesión")
	fmt.Print("Opción: ")

	scanner.Scan()
	switch strings.TrimSpace(scanner.Text()) {
	case "1":
		listarLibrosConsola()
	case "2":
		listarLibrosConsola()
		fmt.Print("\nIngrese el ID del libro que desea leer: ")
		scanner.Scan()
		id, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil {
			fmt.Println("❌ ID inválido.")
			return
		}

		var libroSel *modelo.Libro
		for _, l := range servicioCat.ObtenerTodos() {
			if l.ID == id {
				libroSel = l
				break
			}
		}

		if libroSel == nil {
			fmt.Println("❌ Libro no encontrado.")
			return
		}

		mu.RLock()
		pag := usuarioActivo.ObtenerUltimaPagina(libroSel.ID)
		urlLector := fmt.Sprintf("http://localhost:8080/lector?id=%d&user=%s", libroSel.ID, usuarioActivo.Email)
		mu.RUnlock()

		fmt.Printf("\n📖 Abriendo '%s' en tu navegador...\n", libroSel.Titulo)
		fmt.Printf("📌 Continuando en la página: %d de %d\n", pag, libroSel.PaginasTotales)
		fmt.Printf("🌐 Enlace de lectura: %s\n", urlLector)

		abrirNavegador(urlLector)

	case "3":
		mu.Lock()
		usuarioActivo = nil
		mu.Unlock()
	}
}

func listarLibrosConsola() {
	fmt.Println("\n--- CATÁLOGO DISPONIBLE ---")
	libros := servicioCat.ObtenerTodos()
	if len(libros) == 0 {
		fmt.Println("(No hay libros cargados)")
		return
	}
	for _, l := range libros {
		fmt.Printf("[%d] %s - %s | Género: %s | Págs: %d | Archivo: %s\n", l.ID, l.Titulo, l.Autor, l.Genero, l.PaginasTotales, l.ArchivoPDF)
	}
}

func cargarLibroPDFConsola(scanner *bufio.Scanner) {
	fmt.Println("\n--- CARGA AUTOMÁTICA DE LIBRO POR PDF ---")
	fmt.Print("Ruta del archivo PDF (ej: C:\\docs\\libro.pdf o arrastre el archivo aquí): ")
	scanner.Scan()
	rutaOrigen := strings.Trim(strings.TrimSpace(scanner.Text()), "\"")

	if _, err := os.Stat(rutaOrigen); os.IsNotExist(err) {
		fmt.Println("❌ El archivo especificado no existe.")
		return
	}

	fmt.Println("⏳ Analizando estructura del PDF...")
	totalPaginas, err := contarPaginasPDF(rutaOrigen)
	if err != nil {
		fmt.Printf("❌ Error al procesar PDF: %v\n", err)
		return
	}
	fmt.Printf("📄 Páginas detectadas automáticamente: %d\n", totalPaginas)

	nombreBase := filepath.Base(rutaOrigen)
	tituloSugerido := strings.TrimSuffix(nombreBase, filepath.Ext(nombreBase))

	fmt.Printf("Título [%s]: ", tituloSugerido)
	scanner.Scan()
	titulo := strings.TrimSpace(scanner.Text())
	if titulo == "" {
		titulo = tituloSugerido
	}

	fmt.Print("Autor: ")
	scanner.Scan()
	autor := strings.TrimSpace(scanner.Text())
	if autor == "" {
		autor = "Desconocido"
	}

	fmt.Print("Género: ")
	scanner.Scan()
	genero := strings.TrimSpace(scanner.Text())
	if genero == "" {
		genero = "General"
	}

	nombreDestino := fmt.Sprintf("libro_%d_%s", time.Now().Unix(), nombreBase)
	rutaDestino := filepath.Join(carpetaPDFs, nombreDestino)

	origenFile, err := os.Open(rutaOrigen)
	if err != nil {
		fmt.Printf("❌ Error al abrir archivo origen: %v\n", err)
		return
	}
	defer origenFile.Close()

	destinoFile, err := os.Create(rutaDestino)
	if err != nil {
		fmt.Printf("❌ Error al guardar en uploads: %v\n", err)
		return
	}
	defer destinoFile.Close()

	_, err = io.Copy(destinoFile, origenFile)
	if err != nil {
		fmt.Printf("❌ Error copiando contenido: %v\n", err)
		return
	}

	nuevoID := servicioCat.ObtenerSiguienteID()
	nuevoLibro, err := modelo.NewLibro(nuevoID, titulo, autor, genero, totalPaginas, nombreDestino)
	if err != nil {
		fmt.Printf("❌ Error creando modelo: %v\n", err)
		return
	}

	_ = servicioCat.AgregarLibro(nuevoLibro)
	fmt.Printf("✅ ¡Libro cargado con éxito! ID: %d | Total Páginas: %d\n", nuevoID, totalPaginas)
}

func eliminarLibroConsola(scanner *bufio.Scanner) {
	listarLibrosConsola()
	fmt.Print("\nID del libro a eliminar (0 para cancelar): ")
	scanner.Scan()
	id, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || id == 0 {
		fmt.Println("↩️ Operación cancelada.")
		return
	}

	if err := servicioCat.EliminarLibroPorID(id); err != nil {
		fmt.Printf("❌ %v\n", err)
	} else {
		fmt.Println("🗑️ Libro eliminado exitosamente.")
	}
}