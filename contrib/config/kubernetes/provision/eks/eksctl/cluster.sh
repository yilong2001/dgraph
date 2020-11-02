
#####
# main
##################
main() {
  check_environment
  process_command $@
}

######
# usage - print friendly usage statement
##########################
usage() {
  cat <<-USAGE 1>&2
Create a Kubernetes Cluster with eksctl
Usage:
  $0 [COMMAND] [SUBCOMMAND] [FLAGS] --region [LOCATION] --name [CLUSTER_NAME]

Command:
  create               create EKS cluster using eksctl
  delete               delete EKS cluster created previously with eksctl
  write envvars string create envvar file that can be sourced (default env.sh)
  write config  string create eksctl configuration file (defualt cluster_config.yaml)

USAGE

}

######
# usage_command - print friendly usage statement for delete, create, or write modes
##########################
usage_command() {
  local COMMAND=${1}

  if [[ $COMMAND == "create" ]]; then
    local DESCRIPTION="Create a EKS Cluster with eksctl"
    local CMD="$0 create"
  elif [[ $COMMAND == "delete" ]]; then
    local DESCRIPTION="Delete a EKS Cluster created previously with eksctl"
    local CMD="$0 delete"
  elif [[ $COMMAND == "write" ]]; then
    local SUBCOMMAND=${2}
    if [[ $SUBCOMMAND == "envvars" ]]; then
      local DESCRIPTION="Write env var file (default ./env.sh)"
      local CMD="$0 write envvars"
    elif [[ $SUBCOMMAND == "config" ]]; then
      local DESCRIPTION="Write eksctl cluster config"
      local CMD="$0 write config"
    else
      ## Unknown subcommand
      usage; exit 0
    fi
  else
    ## Unknown Command
    usage; exit 1
  fi

  cat <<-USAGE 1>&2
$DESCRIPTION
Usage:
  $CMD [FLAGS] --region [LOCATION] --name [CLUSTER_NAME]
Flags:
 -a, --addons string         enable IAM Role Polices on Worker Nodes (see https://eksctl.io/usage/schema/)
 -c, --count int             total number of nodes (for a static ASG) (default 3)
 -f, --config-file string    load configuration from a file
 -h, --help                  Help for '$CMD'
 -k, --version string        Kubernetes version (valid options: 1.14, 1.15, 1.16, 1.17) (default "1.17")
 -M, --nodes-max int         maximum nodes in ASG (default 3)
 -m, --nodes-min int         minimum nodes in ASG (default 3)
 -n, --name                  EKS cluster name
 -p, --profile string        AWS credentials profile to use (overrides the AWS_PROFILE environment variable)
 -s, --machine-type string   EC2 node instance type (default "m5.2xlarge")
     --ssh-public-key string SSH public key to use for nodes

USAGE
}

create_cluster() {
  if [[ $DEBUG == "true" ]]; then
    set -ex
  else
    set -e
  fi

  [[ $USAGE == "true" ]] && { usage_command create; exit 0; }
  check_variables

  echo "create_cluster"
}


delete_cluster() {
  if [[ $DEBUG == "true" ]]; then
    set -ex
  else
    set -e
  fi

  [[ $USAGE == "true" ]] && { usage_command delete; exit 0; }
  check_variables


  echo "delete_cluster"
}

######
# write_cluster_config - outputs cluster configuration file
##########################
write_cluster_config() {
  if [[ $DEBUG == "true" ]]; then
    set -ex
  else
    set -e
  fi

  [[ $USAGE == "true" ]] && { usage_command write config; exit 0; }
  check_variables
  local CLUSTER_CONFIG_FILE="${1:-cluster_config.yaml}"

  ## write cluster configuration
  cat <<-CFGEOF | tr -s '\n\n' > $CLUSTER_CONFIG_FILE
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: ${CLUSTER_NAME}
  region: ${CLUSTER_REGION}
  version: "${CLUSTER_K8S_VERSION}"

managedNodeGroups:
  - name: ${CLUSTER_NAME}-workers
    minSize: ${CLUSTER_NODES_MIN}
    maxSize: ${CLUSTER_NODES_MAX}
    desiredCapacity: ${CLUSTER_NODE_COUNT}
    instanceType: ${CLUSTER_MACHINE_TYPE}
    labels: {role: worker}
$(
  [[ -z ${CLUSTER_PUBLIC_KEY_NAME} ]] || \
    printf "    ssh:\n      publicKeyName: ${CLUSTER_PUBLIC_KEY_NAME}\n"
)
    tags:
      nodegroup-role: worker
$(
  if ! [[ -z ${CLUSTER_PUBLIC_KEY_NAME} ]]; then
    printf "    iam:\n      withAddonPolicies:\n"
    for ADDON in $CLUSTER_ADDONS; do
      [[ $ADDON =~ imageBuilder|full-ecr-access ]] && \
        printf "        imageBuilder: true\n"
      [[ $ADDON =~ autoScaler|autoScaler ]] && \
        printf "        autoScaler: true\n"
      [[ $ADDON =~ externalDNS|external-dns-access ]] && \
        printf "        externalDNS: true\n"
      [[ $ADDON == "certManager" ]] && \
        printf "        certManager: true\n"
      [[ $ADDON =~ appMesh|appmesh-access ]] && \
        printf "        appMesh: true\n"
      [[ $ADDON =~ appMeshPreview|appmesh-preview-access ]] && \
        printf "        appMeshPreview: true\n"
      [[ $ADDON == "ebs" ]] && \
        printf "        ebs: true\n"
      [[ $ADDON == "fsx" ]] && \
        printf "        fsx: true\n"
      [[ $ADDON == "efs" ]] && \
        printf "        efs: true\n"
      [[ $ADDON =~ albIngress|alb-ingress-access ]] && \
        printf "        albIngress: true\n"
      [[ $ADDON == "xRay" ]] && \
        printf "        xRay: true\n"
      [[ $ADDON == "cloudWatch" ]] && \
        printf "        cloudWatch: true\n"
    done
  fi
)
CFGEOF
}

######
# write_cluster_envvars - output envvars used to make clsuter config
##########################
write_cluster_envvars() {
  if [[ $DEBUG == "true" ]]; then
    set -ex
  else
    set -e
  fi

  [[ $USAGE == "true" ]] && { usage_command write envvars; exit 0; }
  check_variables
  local CLUSTER_ENV_FILE="${1:-env.sh}"

  cat <<-ENVEOF | tr -s '\n\n' > $CLUSTER_ENV_FILE
export CLUSTER_NAME=${CLUSTER_NAME}
export CLUSTER_REGION=${CLUSTER_REGION}
export CLUSTER_K8S_VERSION="${CLUSTER_K8S_VERSION}"
export CLUSTER_NODES_MIN=${CLUSTER_NODES_MIN}
export CLUSTER_NODES_MAX=${CLUSTER_NODES_MAX}
export CLUSTER_NODE_COUNT=${CLUSTER_NODE_COUNT}
export CLUSTER_MACHINE_TYPE=${CLUSTER_MACHINE_TYPE}
$([[ -z ${CLUSTER_PUBLIC_KEY_NAME} ]] || echo  "export CLUSTER_PUBLIC_KEY_NAME=${CLUSTER_PUBLIC_KEY_NAME}")
export CLUSTER_ADDONS="${CLUSTER_ADDONS}"
ENVEOF
}

######
# write_config - output rendered cluster config or ennvars used to make cluster config
##########################
write_config() {
  local SUBCOMMAND="$1"; shift
  case "$SUBCOMMAND" in
    config) write_cluster_config "$@" ;;
    envvars) write_cluster_envvars "$@" ;;
    *) usage; exit 1 ;;
  esac
}

######
# get_getopt - find GNU getopt or print error message
##########################
get_getopt() {
 unset GETOPT_CMD

 ## Check for GNU getopt compatibility
 if [[ "$(getopt --version)" =~ "--" ]]; then
   local SYSTEM="$(uname -s)"
   if [[ "${SYSTEM,,}" == "freebsd" ]]; then
     ## Check FreeBSD install location
     if [[ -f "/usr/local/bin/getopt" ]]; then
        GETOPT_CMD="/usr/local/bin/getopt"
     else
       ## Save FreeBSD Instructions
       local MESSAGE="On FreeBSD, compatible getopt can be installed with 'sudo pkg install getopt'"
     fi
   elif [[ "${SYSTEM,,}" == "darwin" ]]; then
     ## Check HomeBrew install location
     if [[ -f "/usr/local/opt/gnu-getopt/bin/getopt" ]]; then
        GETOPT_CMD="/usr/local/opt/gnu-getopt/bin/getopt"
     ## Check MacPorts install location
     elif [[ -f "/opt/local/bin/getopt" ]]; then
        GETOPT_CMD="/opt/local/bin/getopt"
     else
        ## Save MacPorts or HomeBrew Instructions
        if command -v brew > /dev/null; then
          local MESSAGE="On macOS, gnu-getopt can be installed with 'brew install gnu-getopt'\n"
        elif command -v port > /dev/null; then
          local MESSAGE="On macOS, getopt can be installed with 'sudo port install getopt'\n"
        fi
     fi
   fi
 else
   GETOPT_CMD="$(command -v getopt)"
 fi

 ## Error if no suitable getopt command found
 if [[ -z $GETOPT_CMD ]]; then
   printf "ERROR: GNU getopt not found.  Please install GNU compatible 'getopt'\n\n%s" "$MESSAGE" 1>&2
   exit 1
 fi
}

#####
# check_environment - check for required commands
##################
check_environment() {
  ## Check for required command line tools
  command -v aws > /dev/null || \
    { echo "[ERROR]: 'aws' command not not found" 1>&2; exit 1; }
  command -v kubectl > /dev/null || \
    { echo "[ERROR]: 'kubectl' command not not found" 1>&2; exit 1; }
  command -v eksctl > /dev/null || \
    { echo "[ERROR]: 'eksctl' command not not found" 1>&2; exit 1; }
}

#####
# check_variables - checks for required variables
##################
check_variables() {
  [[ -z "${CLUSTER_NAME}" ]] && \
    { printf "[ERROR]: 'CLUSTER_NAME' was not defined. Exiting\n" 1>&2; exit 1; }

  [[ -z "${CLUSTER_REGION}" ]] && \
    { printf "[ERROR]: 'CLUSTER_REGION' was not defined. Exiting\n" 1>&2; exit 1; }
}

######
# parse_command - parse command line options using GNU getopt
##########################
process_command() {
  get_getopt
  local GETOPT_SHORT="a:c:f:k:M:n:p:r:s:hd"
  local GETOPT_LONG="addons:,count:,config-file:,version:,nodes-max:,nodes-min:,name:,profile:,region:,machine-type:,help,debug"

  ## Parse Arguments with GNU getopt
  PARSED_ARGUMENTS=$($GETOPT_CMD -o $GETOPT_SHORT --long $GETOPT_LONG -n 'cluster.sh' -- "$@")
  if [ $? != 0 ] ; then usage; exit 1 ; fi
  eval set -- "$PARSED_ARGUMENTS"

  ## General Defaults
  DEBUG="false"

  CLUSTER_NAME="${CLUSTER_NAME}"
  CLUSTER_REGION="${CLUSTER_REGION}"
  CLUSTER_MACHINE_TYPE="${CLUSTER_MACHINE_TYPE:-m5.2xlarge}"
  CLUSTER_NODE_COUNT="${CLUSTER_NODE_COUNT:-3}"
  CLUSTER_NODES_MIN="${CLUSTER_NODES_MIN:-3}"
  CLUSTER_NODES_MAX="${CLUSTER_NODES_MAX:-3}"
  CLUSTER_K8S_VERSION="${CLUSTER_K8S_VERSION:-1.17}"
  CLUSTER_ADDONS="${CLUSTER_ADDONS:-externalDNS certManager albIngress}"
  CLUSTER_SSH_PUBLIC_KEY="${CLUSTER_PUBLIC_KEY_NAME}"
  CLUSTER_SSH_KEY_ACCESS="false"

  ## AWS Specific Defaults
  PROFILE_OVERRIDE=""

  ## Process Agurments
  while true; do
    case "$1" in
      -a | --addons) CLUSTER_ADDONS=$2; shift 2 ;;
      -c | --count) CLUSTER_NODE_COUNT="$2"; shift 2 ;;
      -d | --debug) DEBUG="true"; shift ;;
      -f | --config-file) CONFIG_FILE=$2; shift 2;;
      -k | --version) CLUSTER_K8S_VERSION="$2"; shift 2 ;;
      -h | --help) USAGE="true"; shift;;
      -M | --nodes-max) CLUSTER_NODES_MIN=$2; shift 2;;
      -m | --nodes-min) CLUSTER_NODES_MAX=$2; shift 2;;
      -n | --name) CLUSTER_NAME="$2"; shift 2 ;;
      -p | --profile) PROFILE_OVERRIDE=$2; shift 2 ;;
      -r | --region) CLUSTER_REGION="$2"; shift 2 ;;
      -s | --machine-type) CLUSTER_MACHINE_TYPE=$2; shift 2;;
      --ssh-public-key) CLUSTER_SSH_PUBLIC_KEY="$2"; shift 2;;
      --) shift; break ;;
      *) break ;;
    esac
  done

  ## Process Command
  local COMMAND="$1"; shift
  case "$COMMAND" in
    create) create_cluster "$@" ;;
    delete) delete_cluster "$@" ;;
    write) write_config "$@" ;;
    *) usage; [[ $USAGE == "true" ]] || exit 1 ;;
  esac

}

main "$@"
