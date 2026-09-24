cask "qrouton" do
  version "0.10.0"
  sha256 "7a6f645fc0acf192dc42fc8557f37b29058539f4cd2cbf328808fffd27042ede"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.10.0/qrouton-0.10.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
