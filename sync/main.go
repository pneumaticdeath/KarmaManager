package main

import (
	"bytes"
	_ "embed"
	"html/template"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

const shareBaseURL = "https://karmamanager-sync.fly.dev"

//go:embed pb_public/share.html
var shareTmplSrc string

var shareTmpl = template.Must(template.New("share").Parse(shareTmplSrc))

func main() {
	app := pocketbase.New()

	// Auto-create a user record on first OTP request so sign-up and
	// sign-in are the same single flow (just enter your email).
	app.OnRecordRequestOTPRequest("users").BindFunc(func(e *core.RecordCreateOTPRequestEvent) error {
		if e.Record != nil {
			return e.Next() // user already exists, proceed normally
		}
		// Parse the email from the request body (PocketBase buffers the body
		// so it can be read again here after the main handler already read it).
		var form struct {
			Email string `json:"email" form:"email"`
		}
		if err := e.BindBody(&form); err != nil || form.Email == "" {
			return e.Next() // can't determine email; let standard dummy-200 flow run
		}
		usersCol, err := e.App.FindCollectionByNameOrId("users")
		if err != nil {
			return e.Next()
		}
		record := core.NewRecord(usersCol)
		record.Set("email", form.Email)
		// Set a random password the user will never know — OTP is the only
		// sign-in method, so this password is intentionally inaccessible.
		randomPass := security.RandomString(40)
		record.Set("password", randomPass)
		record.Set("passwordConfirm", randomPass)
		if err := e.App.Save(record); err != nil {
			log.Println("auto-create user failed:", err)
			return e.Next()
		}
		e.Record = record
		return e.Next()
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if err := ensureUsersOTP(app); err != nil {
			log.Println("ensureUsersOTP:", err)
		}
		if err := ensureAppMeta(app); err != nil {
			log.Println("ensureAppMeta:", err)
		}
		if err := ensureFavoritesCollection(app); err != nil {
			log.Println("ensureFavoritesCollection:", err)
		}

		// POST /api/ext/favorites/:id/share — generate share token
		se.Router.POST("/api/ext/favorites/{id}/share", func(e *core.RequestEvent) error {
			id := e.Request.PathValue("id")
			record, err := app.FindRecordById("favorites", id)
			if err != nil {
				return apis.NewNotFoundError("favorite not found", err)
			}
			if record.GetString("user") != e.Auth.Id {
				return apis.NewForbiddenError("access denied", nil)
			}
			token := uuid.New().String()
			record.Set("share_token", token)
			if err := app.Save(record); err != nil {
				return err
			}
			return e.JSON(http.StatusOK, map[string]string{
				"share_url": shareBaseURL + "/share/" + token,
			})
		}).Bind(apis.RequireAuth())

		// DELETE /api/ext/favorites/:id/share — remove share token
		se.Router.DELETE("/api/ext/favorites/{id}/share", func(e *core.RequestEvent) error {
			id := e.Request.PathValue("id")
			record, err := app.FindRecordById("favorites", id)
			if err != nil {
				return apis.NewNotFoundError("favorite not found", err)
			}
			if record.GetString("user") != e.Auth.Id {
				return apis.NewForbiddenError("access denied", nil)
			}
			record.Set("share_token", "")
			if err := app.Save(record); err != nil {
				return err
			}
			return e.JSON(http.StatusOK, map[string]string{"status": "ok"})
		}).Bind(apis.RequireAuth())

		// GET /share/:token — public share page
		se.Router.GET("/share/{token}", func(e *core.RequestEvent) error {
			token := e.Request.PathValue("token")
			record, err := app.FindFirstRecordByFilter("favorites",
				"share_token = {:token} && deleted = false",
				map[string]any{"token": token},
			)
			if err != nil {
				return apis.NewNotFoundError("share link not found", err)
			}
			var buf bytes.Buffer
			if err := shareTmpl.Execute(&buf, map[string]any{
				"Input":   record.GetString("input"),
				"Anagram": record.GetString("anagram"),
			}); err != nil {
				return err
			}
			return e.HTML(http.StatusOK, buf.String())
		})

		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// ensureUsersOTP enables OTP auth on the users collection and keeps
// the length/duration correct even if the collection already existed.
func ensureUsersOTP(app *pocketbase.PocketBase) error {
	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	if col.OTP.Enabled && col.OTP.Length == 6 && col.OTP.Duration == 300 {
		return nil // already correct
	}
	col.OTP.Enabled = true
	col.OTP.Duration = 300 // 5 minutes
	col.OTP.Length = 6
	return app.Save(col)
}

// ensureAppMeta sets the PocketBase app name and email sender name so
// outgoing emails don't say "Acme" / "Support".
func ensureAppMeta(app *pocketbase.PocketBase) error {
	s := app.Settings()
	if s.Meta.AppName == "Acme" || s.Meta.SenderName == "Support" {
		s.Meta.AppName = "Karma Manager"
		s.Meta.SenderName = "Karma Manager"
		return app.Save(s)
	}
	return nil
}

func ensureFavoritesCollection(app *pocketbase.PocketBase) error {
	if _, err := app.FindCollectionByNameOrId("favorites"); err == nil {
		return nil // already exists
	}

	collection := core.NewBaseCollection("favorites")

	collection.Fields.Add(
		&core.TextField{Name: "client_id", Required: true},
	)

	// Resolve the users collection for the relation field.
	usersCol, err := app.FindCollectionByNameOrId("users")
	if err == nil {
		collection.Fields.Add(&core.RelationField{
			Name:         "user",
			Required:     true,
			CollectionId: usersCol.Id,
			MaxSelect:    1,
		})
	}

	collection.Fields.Add(
		&core.TextField{Name: "dictionaries", Required: true},
		&core.TextField{Name: "input", Required: true},
		&core.TextField{Name: "anagram", Required: true},
		&core.TextField{Name: "share_token"},
		&core.BoolField{Name: "deleted"},
	)

	listRule := "user = @request.auth.id"
	viewRule := "user = @request.auth.id || share_token != ''"
	createRule := "@request.auth.id != '' && user = @request.auth.id"
	updateRule := "@request.auth.id != '' && user = @request.auth.id"
	deleteRule := "@request.auth.id != '' && user = @request.auth.id"

	collection.ListRule = &listRule
	collection.ViewRule = &viewRule
	collection.CreateRule = &createRule
	collection.UpdateRule = &updateRule
	collection.DeleteRule = &deleteRule

	return app.Save(collection)
}
