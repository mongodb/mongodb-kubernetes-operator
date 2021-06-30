list_json_parent_files=( ${HOME}/files_modified.json ${HOME}/files_added.json ${HOME}/files_renamed.json)
number_errors=0
echo_with_indent () { echo $1 | sed 's/^/   /';}
get_files_from_json () { cat $1 | jq -r '.[]';}
file_exits () { [[ -f "$1" ]];}

for parent_file in "${list_json_parent_files[@]}"
do
    for file in $(get_files_from_json $parent_file)
    do
        echo "file:" $file
        if (file_exits $file) && [[ $file == *.yaml ]]; then
            output_error=$(kube-linter lint $file 2>&1 > /dev/null)
            output=$(kube-linter lint $file 2> /dev/null)
            exit_statut=$?
            
            if (( $exit_statut > 0)); then
                number_errors=$((number_errors+1))
                echo_with_indent $output
            elif [ -n "$output_error" ]; then
                echo_with_indent $output_error
            else
                echo_with_indent "valid"
            fi
        else
            echo_with_indent "ignored"
        fi
    done    
done
exit $number_errors
