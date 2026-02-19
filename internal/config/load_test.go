package config_test

import (
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
