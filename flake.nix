{
  inputs = {
    # Inputs to be followed by other flakes
    nixpkgs.url = "github:NixOS/nixpkgs/db001797591bf76f7b8d4c4ed3b49233391e0c97";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }@inputs:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; overlays = [ ]; };

        requiredTools = [
          pkgs.go
          pkgs.kubectl
          pkgs.kubectx
          pkgs.kubelogin
          pkgs.kubernetes-controller-tools
          pkgs.kubernetes-helm
        ];

      in
      {
        formatter = pkgs.nixpkgs-fmt;

        # For `nix develop`
        devShells.default = pkgs.mkShellNoCC {
          name = "kro";
          packages = [] ++ requiredTools;
          shellHook = ''
            [ -z $${NIX_DEVELOP_QUIET+x} ] && printf "\n🧙🪄✨ Setting up your development environment... 🧙🪄✨\n\n"
            [ -n $SHELL ] && [[ ! $SHELL =~ ^.*bash.*$ ]] && exec $SHELL
          '';
        };
      });
}
