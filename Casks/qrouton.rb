cask "qrouton" do
  version "0.9.1"
  sha256 "c81a0d81a1298f22255768272f80a5223e3f10a558e2c7cbe7916722a361d845"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.9.1/qrouton-0.9.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
