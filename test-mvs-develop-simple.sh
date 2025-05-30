#!/usr/bin/env bash

DEPOT_PATH=$COSM_DEPOT_PATH

cleanup_pkg(){
    pkg="$1"
    rm -rf "$DEPOT_PATH/examples/$pkg"
}
cleanup_reg(){
    reg="$1"
    rm -rf "$DEPOT_PATH/localhub/$reg"
    cosm registry delete "$reg" --force
}
# ToDo: add a check for validity of the git remote url
remote_add(){
    cwd=$PWD
    # create remote repo
    pkg=$1
    mkdir -p $DEPOT_PATH/localhub/$pkg &> /dev/null;
    cd $DEPOT_PATH/localhub/$pkg
    git init --bare &> /dev/null;
    # add remote to project
    cd "$DEPOT_PATH/examples/$pkg"
    git remote add origin $DEPOT_PATH/localhub/$pkg &> /dev/null;
    git add . &> /dev/null;
    git commit -m "<dep> added dependencies" &> /dev/null;
    git push --set-upstream origin master &> /dev/null;
    cd "$cwd"
}

add_commit_push(){
    cwd=$PWD
    pkg=$1
    cd "$DEPOT_PATH/examples/$pkg"
    git add . &> /dev/null;
    git commit -m "<wip>" &> /dev/null;
    git pull &> /dev/null;
    git push &> /dev/null;
    cd "$cwd"
}

add_pkg_with_git(){
    cwd=$PWD
    pkg=$1
    mkdir "$DEPOT_PATH/examples/$pkg" &> /dev/null;
    cd "$DEPOT_PATH/examples/$pkg"
    git init &> /dev/null;
    cosm init $pkg &> /dev/null;
    git add . &> /dev/null;
    git commit -m "<wip>" &> /dev/null;
    remote_add $pkg &> /dev/null;
    cd "$cwd"
}


# code that runs the test
runall(){
    # create directory for remotes
    mkdir $DEPOT_PATH/localhub &> /dev/null;
    mkdir $DEPOT_PATH/examples &> /dev/null;
    
    # create local registry
    mkdir -p $DEPOT_PATH/localhub/TestRegistry &> /dev/null;
    cd $DEPOT_PATH/localhub/TestRegistry
    git init --bare &> /dev/null;
    cosm registry init TestRegistry $DEPOT_PATH/localhub/TestRegistry

    # root folder in which to create packages
    cd $DEPOT_PATH/examples

    # create packages
    add_pkg_with_git A
    cosm registry add TestRegistry $DEPOT_PATH/localhub/A

    # releases of A
    cd $DEPOT_PATH/examples/A
    cosm release v1.2.3
    cosm release --patch    # v1.2.4
    cosm release --minor    # v1.3.0
    cosm release --major    # v2.0.0
    
    # add a single release
    cosm registry add TestRegistry A v1.3.0
}

cleanall(){
    cleanup_pkg A
    cleanup_reg TestRegistry
    rm -rf $DEPOT_PATH/clones/*
    rm -rf $DEPOT_PATH/packages/*
    rm -rf $DEPOT_PATH/localhub/*
}

# no input arguments - run test and cleanup
if [ "$#" == 0 ]; then
    cleanall
    runall
fi

# run test  or cleanup
if [ "$#" == 1 ]; then
    case "$1" in
        --run)
            runall
            ;;
        --clean)
            cleanall
            ;;
        *)
            printf "Wrong input arguments. Prodide '--run' and or 'clean'. \n \n"
            exit 1
            ;;
    esac
fi

exit 0