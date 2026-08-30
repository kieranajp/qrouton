cask "qrouton" do
  version "0.4.1"
  sha256 "84b924bdc89ad2a4e8b1ed086e0c1352333309dd9188a8742ba399c6812bc09f"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.4.1/qrouton-0.4.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
