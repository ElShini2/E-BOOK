package catalogo

import (
	"e-book/modelo"
	"errors"
	"strings"
)

// GestorCatalogo define la INTERFAZ que establece el contrato de operaciones
// que cualquier sistema de catálogo debe implementar para consultar e-books.
type GestorCatalogo interface {
	BuscarPorTitulo(titulo string) (*modelo.Libro, error)
	FiltrarPorGenero(genero string) ([]*modelo.Libro, error)
	FiltrarPorAutor(autor string) ([]*modelo.Libro, error)
	ObtenerTodos() []*modelo.Libro
}

// ServicioCatalogo es la estructura que implementa la interfaz GestorCatalogo.
// Almacena la colección de libros en memoria mediante un slice inmutable de punteros.
type ServicioCatalogo struct {
	libros []*modelo.Libro
}

// NewServicioCatalogo es el CONSTRUCTOR que inicializa el catálogo con una lista base de libros.
func NewServicioCatalogo(librosIniciales []*modelo.Libro) *ServicioCatalogo {
	return &ServicioCatalogo{
		libros: librosIniciales,
	}
}

// BuscarPorTitulo busca un libro por coincidencia exacta o parcial de su título.
// Devuelve un error nativo si el título ingresado es inválido o si no encuentra el libro.
func (s *ServicioCatalogo) BuscarPorTitulo(titulo string) (*modelo.Libro, error) {
	// MANEJO DE ERRORES: Validación de entrada
	if strings.TrimSpace(titulo) == "" {
		return nil, errors.New("el término de búsqueda no puede estar vacío")
	}

	tituloLwr := strings.ToLower(titulo)

	// Recorremos el slice del catálogo
	for _, l := range s.libros {
		if strings.Contains(strings.ToLower(l.Titulo), tituloLwr) {
			return l, nil // Retorno exitoso: (puntero, nil)
		}
	}

	// MANEJO DE ERRORES: Libro no encontrado
	return nil, errors.New("no se encontró ningún libro con el título especificado")
}

// FiltrarPorGenero retorna una colección de libros pertenecientes a una categoría/género.
func (s *ServicioCatalogo) FiltrarPorGenero(genero string) ([]*modelo.Libro, error) {
	if strings.TrimSpace(genero) == "" {
		return nil, errors.New("debe especificar un género válido para filtrar")
	}

	var resultado []*modelo.Libro
	generoLwr := strings.ToLower(genero)

	for _, l := range s.libros {
		if strings.ToLower(l.Genero) == generoLwr {
			resultado = append(resultado, l)
		}
	}

	// MANEJO DE ERRORES: Colección vacía
	if len(resultado) == 0 {
		return nil, errors.New("no existen libros registrados bajo el género ingresado")
	}

	return resultado, nil
}

// FiltrarPorAutor retorna todos los libros escritos por un autor determinado.
func (s *ServicioCatalogo) FiltrarPorAutor(autor string) ([]*modelo.Libro, error) {
	if strings.TrimSpace(autor) == "" {
		return nil, errors.New("el nombre del autor no puede estar vacío")
	}

	var resultado []*modelo.Libro
	autorLwr := strings.ToLower(autor)

	for _, l := range s.libros {
		if strings.Contains(strings.ToLower(l.Autor), autorLwr) {
			resultado = append(resultado, l)
		}
	}

	if len(resultado) == 0 {
		return nil, errors.New("no se encontraron obras asociadas al autor indicado")
	}

	return resultado, nil
}

// ObtenerTodos retorna la lista completa de libros almacenados en el catálogo.
func (s *ServicioCatalogo) ObtenerTodos() []*modelo.Libro {
	return s.libros
}
