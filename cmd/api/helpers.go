package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/htet-29/greenlight/internal/custom"
	"github.com/htet-29/greenlight/internal/data"
	"github.com/htet-29/greenlight/internal/domain"
	"github.com/htet-29/greenlight/internal/validator"
	"github.com/julienschmidt/httprouter"
)

// readIDParam reads 'id' parameter from request context and
// parse it to base 10, 64 bit integer
func (app *application) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)

	if err != nil || id < 1 {
		return 0, errors.New("invalid id paramter")
	}

	return id, nil
}

// envelope is a type for enveloping json response
type envelope map[string]any

// writeJSON is a helper for sending responses.
func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// limit the size of the request to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	dec := json.NewDecoder(r.Body)
	// Return an error instead of silen ignoring the field when client
	// includes any field that cannot be mapped to the target destination
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		// JSON syntax error
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %v)", syntaxError.Offset)

		// JSON syntax error
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		// JSON value is the wrong type for the target destination
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %v)", unmarshalTypeError.Offset)

		// If the request body is empty
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		// If the JSON contains a field which cannot be mapped to the target destination
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		// If the error has the type *http.MaxBytesError
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)

		// if we pass something that is not non-nil pointer as target destination
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}

	// Check if there is additional data in the request body, if there is return error
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

// toDomainMovie is a helper to convert database model to
// domain(business) model.
func toDomainMovie(dbMovie data.Movie) domain.Movie {
	return domain.Movie{
		ID:        dbMovie.ID,
		Title:     dbMovie.Title,
		Year:      dbMovie.Year,
		Runtime:   custom.Runtime(dbMovie.Runtime),
		Genres:    dbMovie.Genres,
		Version:   dbMovie.Version,
		CreatedAt: dbMovie.CreatedAt.Time,
	}
}

// readString is a helper that returns a string value from the query string, or the provided
// default value if no matching key could be found
func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	return s
}

// readCSV is a helper that reads a string value from the query string and then splits it
// into a slice on the comma character. If no matching key could be found, it returns
// the provided default value.
func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)

	if csv == "" {
		return defaultValue
	}

	return strings.Split(csv, ",")
}

// readInt is a helper that reads a string value from the query string and converts it to an
// integer before returning. If no matching key could be found it returns the provided
// default value. If the value couldn't be converted to an integer, then we record an
// error message in the provided Validator instance.
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	// Try to convert the value to an int. If this fails, add an error message to the
	// validator instance and return the default value.
	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be a integer value")
		return defaultValue
	}

	return i
}
