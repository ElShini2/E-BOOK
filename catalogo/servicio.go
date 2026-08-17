package catalogo

import (
	"e-book/modelo"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
)

const archivoLibros = "libros.json"

// ServicioCatalogo gestiona la colección de e-books con persistencia en disco y control concurrente.
type ServicioCatalogo struct {
	libros []*modelo.Libro
	mu     sync.RWMutex
}

// NewServicioCatalogo inicializa el catálogo cargando los datos guardados en disco.
func NewServicioCatalogo() *ServicioCatalogo {
	sc := &ServicioCatalogo{
		libros: make([]*modelo.Libro, 0),
	}
	_ = sc.cargarDesdeDisco()
	return sc
}

// GuardarEnDisco serializa el slice de libros a JSON legible en disco.
func (s *ServicioCatalogo) GuardarEnDisco() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	datos, err := json.MarshalIndent(s.libros, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(archivoLibros, datos, 0644)
}

// cargarDesdeDisco lee el archivo JSON al iniciar la aplicación.
func (s *ServicioCatalogo) cargarDesdeDisco() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(archivoLibros); os.IsNotExist(err) {
		return nil
	}

	datos, err := os.ReadFile(archivoLibros)
	if err != nil {
		return err
	}

	return json.Unmarshal(datos, &s.libros)
}

// ObtenerSiguienteID calcula el ID más alto existente y suma 1 para evitar duplicados.
func (s *ServicioCatalogo) ObtenerSiguienteID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	maxID := 0
	for _, l := range s.libros {
		if l.ID > maxID {
			maxID = l.ID
		}
	}
	return maxID + 1
}

// AgregarLibro añade un nuevo libro y sincroniza el archivo en disco.
func (s *ServicioCatalogo) AgregarLibro(l *modelo.Libro) error {
	if l == nil {
		return errors.New("no se puede agregar un libro nulo")
	}

	s.mu.Lock()
	s.libros = append(s.libros, l)
	s.mu.Unlock()

	return s.GuardarEnDisco()
}

// EliminarLibroPorID busca un libro por ID, lo remueve y sincroniza el archivo en disco.
func (s *ServicioCatalogo) EliminarLibroPorID(id int) error {
	s.mu.Lock()
	encontrado := false
	for i, l := range s.libros {
		if l.ID == id {
			s.libros = append(s.libros[:i], s.libros[i+1:]...)
			encontrado = true
			break
		}
	}
	s.mu.Unlock()

	if !encontrado {
		return errors.New("no se encontró ningún libro con el ID especificado")
	}

	return s.GuardarEnDisco()
}

// ObtenerTodos retorna una copia del listado de libros disponibles.
func (s *ServicioCatalogo) ObtenerTodos() []*modelo.Libro {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.libros
}

// BuscarPorTitulo busca un libro por coincidencia insensible a mayúsculas/minúsculas.
func (s *ServicioCatalogo) BuscarPorTitulo(titulo string) (*modelo.Libro, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tLower := strings.ToLower(strings.TrimSpace(titulo))
	for _, l := range s.libros {
		if strings.Contains(strings.ToLower(l.Titulo), tLower) {
			return l, nil
		}
	}
	return nil, errors.New("libro no encontrado en el catálogo")
}
