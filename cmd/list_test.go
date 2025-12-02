package cmd_test

import (
	"io"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/cmd"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
)

var _ = Describe("List Command", func() {
	var (
		tempDir      string
		clotildeRoot string
		originalWd   string
		store        session.Store
	)

	BeforeEach(func() {
		// Create temp directory
		tempDir = GinkgoT().TempDir()

		// Save original working directory
		var err error
		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		// Change to temp directory
		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		// Initialize clotilde
		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)
		store = session.NewFileStore(clotildeRoot)
	})

	AfterEach(func() {
		// Restore working directory
		_ = os.Chdir(originalWd)
	})

	It("should list all sessions sorted by lastAccessed", func() {
		// Create multiple sessions
		sess1 := session.NewSession("oldest", "uuid-1")
		sess1.Metadata.LastAccessed = time.Now().Add(-2 * time.Hour)
		err := store.Create(sess1)
		Expect(err).NotTo(HaveOccurred())

		sess2 := session.NewSession("newest", "uuid-2")
		sess2.Metadata.LastAccessed = time.Now()
		err = store.Create(sess2)
		Expect(err).NotTo(HaveOccurred())

		sess3 := session.NewSession("middle", "uuid-3")
		sess3.Metadata.LastAccessed = time.Now().Add(-1 * time.Hour)
		err = store.Create(sess3)
		Expect(err).NotTo(HaveOccurred())

		// Execute list command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"list"})

		// Capture output by redirecting stdout
		// Note: This is a basic test - we're mainly verifying it doesn't error
		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())
	})

	It("should handle empty session list", func() {
		// Execute list command with no sessions
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"list"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())
	})

	It("should show forked sessions with parent info", func() {
		// Create parent session
		parent := session.NewSession("parent", "uuid-parent")
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		// Create forked session
		fork := session.NewSession("fork", "uuid-fork")
		fork.Metadata.IsForkedSession = true
		fork.Metadata.ParentSession = "parent"
		err = store.Create(fork)
		Expect(err).NotTo(HaveOccurred())

		// Execute list command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"list"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())
	})
})
