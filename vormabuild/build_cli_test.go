package vormabuild

import (
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestBuildWithOptions(t *testing.T) {
	t.Run("returns error when app is nil", func(t *testing.T) {
		err := BuildWithOptions(nil, BuildOptions{})
		if err == nil {
			t.Fatal("expected BuildWithOptions to return an error for nil app")
		}
	})

	t.Run("runs hook-inner dev mode through public facade", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		t.Chdir(fixture.RootDir)
		testkit.WriteBootstrapStyleRoutesFixtureFiles(t)

		app := vorma.NewVormaApp(vorma.VormaAppConfig{
			Wave: fixture.App.Wave,
		})

		err := BuildWithOptions(
			app,
			BuildOptions{
				Dev:               true,
				HookExecutionOnly: true,
			},
		)
		if err != nil {
			t.Fatalf("BuildWithOptions returned error: %v", err)
		}
		if app.BuildID() == "" {
			t.Fatal("expected BuildWithOptions hook-inner dev mode to assign a build ID")
		}
	})
}
