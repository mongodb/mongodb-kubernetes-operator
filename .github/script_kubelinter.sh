array=( ${HOME}/files_modified.json ${HOME}/files_added.json ${HOME}/files_renamed.json)
number_errors=0
indent() { sed 's/^/    /'; }
for parent_file in "${array[@]}"
do
    for file in $(cat $parent_file | jq -r '.[]')
    do
        echo "file:" $file
        if [[ $file == *.yaml ]] && [[ -f "$file" ]]; then
            output_error=$(kube-linter lint $file 2>&1 > /dev/null)
            output=$(kube-linter lint $file 2> /dev/null)
            exit_statut=$?
            if (( $exit_statut > 0)); then
                number_errors=$((number_errors+1))
                echo $output | sed 's/^/    /'
            elif [ -n "$output_error" ]; then
                echo $output_error | indent
            else
                echo "Valid" | indent
            fi
        else
            echo "ignored" | indent
        fi
    done    
done
exit $number_errors