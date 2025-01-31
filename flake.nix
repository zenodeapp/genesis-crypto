{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/release-22.11";
    flake-utils.url = "github:numtide/flake-utils";
    nix-bundle-exe = {
      url = "github:3noch/nix-bundle-exe";
      flake = false;
    };
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.utils.follows = "flake-utils";
    };
  };

  outputs = { self, nixpkgs, nix-bundle-exe, gomod2nix, flake-utils }:
    let
      rev = self.shortRev or "dirty";
      mkApp = drv: {
        type = "app";
        program = "${drv}/bin/${drv.meta.mainProgram}";
      };
    in
    (flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [
              (import ./nix/build_overlay.nix)
              gomod2nix.overlays.default
              self.overlay
            ];
            config = { };
          };
        in
        rec {
          packages = pkgs.genesis-matrix // {
            inherit (pkgs) rocksdb;
          };
          apps = {
            genesisd = mkApp packages.genesisd;
            genesisd-testnet = mkApp packages.genesisd-testnet;
          };
          defaultPackage = packages.genesisd;
          defaultApp = apps.genesisd;
          devShells = {
            genesisd = pkgs.mkShell {
              buildInputs = with pkgs; [
                go_1_20
                rocksdb
                gomod2nix
              ];
            };
          };
          devShell = devShells.genesisd;
          legacyPackages = pkgs;
        }
      )
    ) // {
      overlay = final: super: {
        bundle-exe = final.pkgsBuildBuild.callPackage nix-bundle-exe { };
        # make-tarball don't follow symbolic links to avoid duplicate file, the bundle should have no external references.
        # reset the ownership and permissions to make the extract result more normal.
        make-tarball = drv: final.runCommand "tarball-${drv.name}"
          {
            nativeBuildInputs = with final.buildPackages; [ gnutar gzip ];
          } ''
          tar cfv - -C "${drv}" \
            --owner=0 --group=0 --mode=u+rw,uga+r --hard-dereference . \
            | gzip -9 > $out
        '';
        bundle-win-exe = drv: final.callPackage ./nix/bundle-win-exe.nix { genesisd = drv; };
      } // (with final;
        let
          matrix = lib.cartesianProductOfSets {
            network = [ "mainnet" "testnet" ];
            pkgtype = [
              "nix" # normal nix package
              "bundle" # relocatable bundled package
              "tarball" # tarball of the bundle, for distribution and checksum
            ];
          };
          binaries = builtins.listToAttrs (builtins.map
            ({ network, pkgtype }: {
              name = builtins.concatStringsSep "-" (
                [ "genesisd" ] ++
                lib.optional (network != "mainnet") network ++
                lib.optional (pkgtype != "nix") pkgtype
              );
              value =
                let
                  genesisd = callPackage ./. {
                    inherit rev network;
                  };
                  bundle =
                    if stdenv.hostPlatform.isWindows then
                      bundle-win-exe genesisd
                    else
                      bundle-exe genesisd;
                in
                if pkgtype == "bundle" then
                  bundle
                else if pkgtype == "tarball" then
                  make-tarball bundle
                else
                  genesisd;
            })
            matrix
          );
        in
        {
          genesis-matrix = binaries;
        }
      );
    };
}
