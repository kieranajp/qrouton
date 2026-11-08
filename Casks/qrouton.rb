cask "qrouton" do
  version "0.17.1"
  sha256 "bc254a1b3d8e1794fd57d1a1f532218825efb6b390134e2bc7fd145d67b66855"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.17.1/qrouton-0.17.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
