cask "qrouton" do
  version "0.15.1"
  sha256 "f4711aaf2e29f872d2a5de2be5d938b786536ce075b5804fb293188385f148f4"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.15.1/qrouton-0.15.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
