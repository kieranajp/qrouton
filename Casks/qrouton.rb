cask "qrouton" do
  version "0.13.2"
  sha256 "aa01aebf700727a9088fbb092d050b2644d30293c64659b173ee593b28b6defd"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.13.2/qrouton-0.13.2-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
