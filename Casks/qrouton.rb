cask "qrouton" do
  version "0.7.0"
  sha256 "65608d16bafc495fa1730c245c0393eece2166a00bedeeaa0353957974a116b8"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.7.0/qrouton-0.7.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
