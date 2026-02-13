package tooling

import "github.com/vormadev/vorma/lab/vitecmd"

// ViteProdBuild runs a Vite production build
func (b *Builder) ViteProdBuild() error {
	if !b.cfg.UsingVite() {
		return nil
	}
	return b.viteCtx().ProdBuild()
}

func (b *Builder) viteCtx() *vitecmd.BuildCtx {
	return vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
		JSPackageManagerBaseCmd: b.cfg.Vite.JSPackageManagerBaseCmd,
		JSPackageManagerCmdDir:  b.cfg.Vite.JSPackageManagerCmdDir,
		OutDir:                  b.cfg.Dist.StaticPublic(),
		ManifestOut:             b.cfg.ViteManifestPath(),
		ViteConfigFile:          b.cfg.Vite.ViteConfigFile,
		DefaultPort:             b.cfg.Vite.DefaultPort,
	})
}

// NewViteDevContext creates a new Vite dev context
func (b *Builder) NewViteDevContext() (*vitecmd.BuildCtx, error) {
	if !b.cfg.UsingVite() {
		return nil, nil
	}
	ctx := b.viteCtx()
	if err := ctx.DevBuild(); err != nil {
		return nil, err
	}
	return ctx, nil
}
