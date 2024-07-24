{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = [
    pkgs.go
  ];

  shellHook = ''
    echo "Go development environment loaded"
    go version
  '';
}
