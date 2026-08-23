package usuario

import (
	"e-book/modelo"
	"errors"
	"fmt"
	"strings"
)

type RolUsuario string

const (
	RolAdmin   RolUsuario = "ADMIN"
	RolCliente RolUsuario = "CLIENTE"
)

type Usuario struct {
	ID              int             `json:"id"`
	Nombre          string          `json:"nombre"`
	Email           string          `json:"email"`
	Password        string          `json:"password"`
	Rol             RolUsuario      `json:"rol"`
	Favoritos       []*modelo.Libro `json:"favoritos"`
	ProgresoLectura map[string]int  `json:"progreso_lectura"` // Llave en string para total compatibilidad con JSON
}

func NewUsuario(id int, nombre, email, password string, rol RolUsuario) (*Usuario, error) {
	if strings.TrimSpace(nombre) == "" || strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
		return nil, errors.New("los datos del usuario no pueden estar vacíos")
	}

	return &Usuario{
		ID:              id,
		Nombre:          nombre,
		Email:           strings.ToLower(strings.TrimSpace(email)),
		Password:        password,
		Rol:             rol,
		Favoritos:       make([]*modelo.Libro, 0),
		ProgresoLectura: make(map[string]int),
	}, nil
}

func (u *Usuario) ValidarPassword(password string) bool {
	return u.Password == password
}

func (u *Usuario) ActualizarProgreso(libroID int, pagina int) {
	if u.ProgresoLectura == nil {
		u.ProgresoLectura = make(map[string]int)
	}
	key := fmt.Sprintf("%d", libroID)
	u.ProgresoLectura[key] = pagina
}

func (u *Usuario) ObtenerUltimaPagina(libroID int) int {
	if u.ProgresoLectura == nil {
		return 1
	}
	key := fmt.Sprintf("%d", libroID)
	if pag, ok := u.ProgresoLectura[key]; ok && pag > 0 {
		return pag
	}
	return 1
}

func (u *Usuario) AgregarFavorito(l *modelo.Libro) error {
	if l == nil {
		return errors.New("el libro no puede ser nulo")
	}
	for _, f := range u.Favoritos {
		if f.ID == l.ID {
			return errors.New("el libro ya está en favoritos")
		}
	}
	u.Favoritos = append(u.Favoritos, l)
	return nil
}

func (u *Usuario) ObtenerFavoritos() []*modelo.Libro {
	return u.Favoritos
}