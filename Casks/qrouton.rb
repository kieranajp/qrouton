cask "qrouton" do
  version "0.9.0"
  sha256 "fa05b1ef82e4150f937bb9350f6ab94647566b4cbb9e1c74030a510ac8c24668"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.9.0/qrouton-0.9.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
