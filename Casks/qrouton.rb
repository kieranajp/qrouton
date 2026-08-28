cask "qrouton" do
  version "0.3.1"
  sha256 "9f8e9124d1f9bfbb8b8765cddc11911c2118c3c117bae93998af6bca513a8f4e"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.3.1/qrouton-0.3.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
