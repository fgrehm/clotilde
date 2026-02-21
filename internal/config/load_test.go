package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("Load and Save", func() {
	var tempDir string
	var clotildeRoot string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)
		err := util.EnsureDir(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should save and load config", func() {
		cfg := &config.Config{
			Profiles: map[string]config.Profile{
				"quick": {
					Model: "haiku",
				},
			},
		}

		err := config.Save(clotildeRoot, cfg)
		Expect(err).NotTo(HaveOccurred())

		loaded, err := config.Load(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Profiles["quick"].Model).To(Equal("haiku"))
	})

	It("should return error if config file doesn't exist", func() {
		_, err := config.Load(clotildeRoot)
		Expect(err).To(HaveOccurred())
	})

	It("should preserve empty fields", func() {
		cfg := config.NewConfig()
		// Profiles is empty map

		err := config.Save(clotildeRoot, cfg)
		Expect(err).NotTo(HaveOccurred())

		loaded, err := config.Load(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Profiles).To(BeEmpty())
	})
})

var _ = Describe("LoadOrDefault", func() {
	var tempDir string
	var clotildeRoot string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)
		err := util.EnsureDir(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should load existing config", func() {
		cfg := &config.Config{
			Profiles: map[string]config.Profile{
				"research": {
					Model: "opus",
				},
			},
		}
		err := config.Save(clotildeRoot, cfg)
		Expect(err).NotTo(HaveOccurred())

		loaded, err := config.LoadOrDefault(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Profiles["research"].Model).To(Equal("opus"))
	})

	It("should return default config if file doesn't exist", func() {
		loaded, err := config.LoadOrDefault(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded).NotTo(BeNil())
		Expect(loaded.Profiles).To(BeEmpty())
	})
})

var _ = Describe("NewConfig", func() {
	It("should create config with defaults", func() {
		cfg := config.NewConfig()
		Expect(cfg).NotTo(BeNil())
		Expect(cfg.Profiles).To(BeEmpty())
	})
})

var _ = Describe("LoadGlobalOrDefault", func() {
	var origXDG string

	BeforeEach(func() {
		origXDG = os.Getenv("XDG_CONFIG_HOME")
	})

	AfterEach(func() {
		if origXDG == "" {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDG)
		}
	})

	It("returns empty config when file is absent", func() {
		tmpDir := GinkgoT().TempDir()
		_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

		cfg, err := config.LoadGlobalOrDefault()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())
		Expect(cfg.Profiles).To(BeEmpty())
	})

	It("loads profiles correctly when file is present", func() {
		tmpDir := GinkgoT().TempDir()
		_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

		globalDir := filepath.Join(tmpDir, "clotilde")
		Expect(os.MkdirAll(globalDir, 0o755)).To(Succeed())
		data, _ := json.Marshal(map[string]interface{}{
			"profiles": map[string]interface{}{
				"quick": map[string]string{"model": "haiku"},
			},
		})
		Expect(os.WriteFile(filepath.Join(globalDir, "config.json"), data, 0o644)).To(Succeed())

		cfg, err := config.LoadGlobalOrDefault()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Profiles["quick"].Model).To(Equal("haiku"))
	})
})

var _ = Describe("MergedProfiles", func() {
	var tmpDir string
	var clotildeRoot string
	var origXDG string

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
		clotildeRoot = filepath.Join(tmpDir, "project", config.ClotildeDir)
		Expect(util.EnsureDir(clotildeRoot)).To(Succeed())

		origXDG = os.Getenv("XDG_CONFIG_HOME")
		globalDir := filepath.Join(tmpDir, "xdg")
		Expect(os.MkdirAll(filepath.Join(globalDir, "clotilde"), 0o755)).To(Succeed())
		_ = os.Setenv("XDG_CONFIG_HOME", globalDir)
	})

	AfterEach(func() {
		if origXDG == "" {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDG)
		}
	})

	writeGlobalConfig := func(profiles map[string]config.Profile) {
		globalDir := os.Getenv("XDG_CONFIG_HOME")
		data, err := json.Marshal(&config.Config{Profiles: profiles})
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(globalDir, "clotilde", "config.json"), data, 0o644)).To(Succeed())
	}

	It("returns global profiles when only global config has profiles", func() {
		writeGlobalConfig(map[string]config.Profile{
			"quick": {Model: "haiku"},
		})

		merged, err := config.MergedProfiles(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(HaveKey("quick"))
		Expect(merged["quick"].Model).To(Equal("haiku"))
	})

	It("returns project profiles when only project config has profiles", func() {
		err := config.Save(clotildeRoot, &config.Config{
			Profiles: map[string]config.Profile{
				"strict": {PermissionMode: "ask"},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		merged, err := config.MergedProfiles(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(HaveKey("strict"))
		Expect(merged["strict"].PermissionMode).To(Equal("ask"))
	})

	It("project profile overrides global profile with same name", func() {
		writeGlobalConfig(map[string]config.Profile{
			"quick": {Model: "haiku"},
		})
		err := config.Save(clotildeRoot, &config.Config{
			Profiles: map[string]config.Profile{
				"quick": {Model: "opus"},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		merged, err := config.MergedProfiles(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged["quick"].Model).To(Equal("opus"))
	})

	It("returns empty map when neither config has profiles", func() {
		merged, err := config.MergedProfiles(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(BeEmpty())
	})

	It("includes profiles from both configs when names differ", func() {
		writeGlobalConfig(map[string]config.Profile{
			"quick": {Model: "haiku"},
		})
		err := config.Save(clotildeRoot, &config.Config{
			Profiles: map[string]config.Profile{
				"strict": {PermissionMode: "ask"},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		merged, err := config.MergedProfiles(clotildeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(HaveKey("quick"))
		Expect(merged).To(HaveKey("strict"))
	})
})
