package modelo

import (
	"errors"
	"fmt"
)

// Libro representa la estructura de un e-book dentro de la biblioteca.
// Los campos inician en Mayúscula para permitir su exportación/serialización.
type Libro struct {
	ID             int    `json:"id"`
	Titulo         string `json:"titulo"`
	Autor          string `json:"autor"`
	Genero         string `json:"genero"`
	PaginasTotales int    `json:"paginas_totales"`
	paginasLeidas  int    // Campo PRIVADO (encapsulado): solo accesible mediante métodos
}

// NewLibro es un CONSTRUCTOR PERSONALIZADO que valida los datos iniciales
// y retorna un puntero (*Libro) a la nueva instancia creada.
func NewLibro(id int, titulo, autor, genero string, paginasTotales int) (*Libro, error) {
	if paginasTotales <= 0 {
		return nil, errors.New("el número de páginas totales debe ser mayor a cero")
	}
	if titulo == "" || autor == "" {
		return nil, errors.New("el título y el autor no pueden estar vacíos")
	}

	return &Libro{
		ID:             id,
		Titulo:         titulo,
		Autor:          autor,
		Genero:         genero,
		PaginasTotales: paginasTotales,
		paginasLeidas:  0, // Inicializa encapsulado en 0
	}, nil
}

// ActualizarProgreso es un MÉTODO con receptor de puntero (*Libro)
// Permite modificar el estado encapsulado del campo privado paginasLeidas.
func (l *Libro) ActualizarProgreso(paginas int) error {
	// MANEJO DE ERRORES: Validación de límites
	if paginas < 0 {
		return errors.New("el progreso de páginas no puede ser negativo")
	}
	if paginas > l.PaginasTotales {
		return fmt.Errorf("las páginas leídas (%d) no pueden superar las totales (%d)", paginas, l.PaginasTotales)
	}

	// Asignación segura del estado privado
	l.paginasLeidas = paginas
	return nil
}

// PaginasLeidas es un GETTER que permite consultar el valor del campo privado paginasLeidas.
func (l *Libro) PaginasLeidas() int {
	return l.paginasLeidas
}
