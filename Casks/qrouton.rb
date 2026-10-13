cask "qrouton" do
  version "0.15.0"
  sha256 "cc0a79439d446e66f092198aaa40f5c837b420aa67277f267c159113b120ddbe"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.15.0/qrouton-0.15.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
