package usuario

import (
	"e-book/modelo"
	"errors"
	"strings"
)

type RolUsuario string

const (
	RolAdmin   RolUsuario = "ADMIN"
	RolCliente RolUsuario = "CLIENTE"
)

type Usuario struct {
	ID        int             `json:"id"`
	Nombre    string          `json:"nombre"`
	Email     string          `json:"email"`
	Password  string          `json:"password"` // Campo exportado para persistencia en JSON
	Rol       RolUsuario      `json:"rol"`
	Favoritos []*modelo.Libro `json:"favoritos"`
}

// NewUsuario crea un nuevo usuario validando datos básicos.
func NewUsuario(id int, nombre, email, password string, rol RolUsuario) (*Usuario, error) {
	if strings.TrimSpace(nombre) == "" || strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
		return nil, errors.New("los datos del usuario no pueden estar vacíos")
	}

	return &Usuario{
		ID:        id,
		Nombre:    nombre,
		Email:     strings.ToLower(strings.TrimSpace(email)),
		Password:  password,
		Rol:       rol,
		Favoritos: make([]*modelo.Libro, 0),
	}, nil
}

// ValidarPassword comprueba si la contraseña coincide.
func (u *Usuario) ValidarPassword(password string) bool {
	return u.Password == password
}

// AgregarFavorito añade un libro al listado de favoritos evitando duplicados.
func (u *Usuario) AgregarFavorito(l *modelo.Libro) error {
	if l == nil {
		return errors.New("el libro no puede ser nulo")
	}
	for _, f := range u.Favoritos {
		if f.ID == l.ID {
			return errors.New("el libro ya está en tu lista de favoritos")
		}
	}
	u.Favoritos = append(u.Favoritos, l)
	return nil
}

// ObtenerFavoritos retorna la lista de favoritos del usuario.
func (u *Usuario) ObtenerFavoritos() []*modelo.Libro {
	return u.Favoritos
}
