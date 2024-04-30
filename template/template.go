package template

import (
	"errors"
	"io"
	"net/http"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

type Template struct{}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	component, valid := data.(templ.Component)
	if !valid {
		return errors.New("data value is not valid templ component")
	}

	return component.Render(c.Request().Context(), w)
}

func NewTemplateRenderer(e *echo.Echo) {
	t := newTemplate()
	e.Renderer = t
}

func newTemplate() echo.Renderer {
	return &Template{}
}

func Html(c echo.Context, component templ.Component) error {
	return c.Render(http.StatusOK, "", component)
}
