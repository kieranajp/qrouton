cask "qrouton" do
  version "0.10.1"
  sha256 "9e9dcc6491a09f64e040abd2bea16a703d8056de56db12e953389256ed6e882c"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.10.1/qrouton-0.10.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
