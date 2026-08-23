package modelo

import (
	"errors"
	"strings"
)

type Libro struct {
	ID             int    `json:"id"`
	Titulo         string `json:"titulo"`
	Autor          string `json:"autor"`
	Genero         string `json:"genero"`
	PaginasTotales int    `json:"paginas_totales"`
	ArchivoPDF     string `json:"archivo_pdf"` // Ruta o nombre del archivo PDF guardado
}

func NewLibro(id int, titulo, autor, genero string, paginas int, archivoPDF string) (*Libro, error) {
	if strings.TrimSpace(titulo) == "" {
		return nil, errors.New("el título no puede estar vacío")
	}
	if strings.TrimSpace(autor) == "" {
		return nil, errors.New("el autor no puede estar vacío")
	}
	if strings.TrimSpace(genero) == "" {
		return nil, errors.New("el género no puede estar vacío")
	}
	if paginas <= 0 {
		return nil, errors.New("el número de páginas debe ser mayor a 0")
	}

	return &Libro{
		ID:             id,
		Titulo:         strings.TrimSpace(titulo),
		Autor:          strings.TrimSpace(autor),
		Genero:         strings.TrimSpace(genero),
		PaginasTotales: paginas,
		ArchivoPDF:     archivoPDF,
	}, nil
}