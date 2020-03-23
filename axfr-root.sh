#! /usr/bin/env bash

DATE=$(date +%Y-%m-%d)
SAVEDIR="./"

export DATE
export SAVEDIR

PARALLEL=20

# run in parallel using sem
function runp {
    sem -j "$PARALLEL" --id "$$" "$@"
}


function zoneTransfer {
    HOST=$1
    ZONE=$2
    OUT=$3
    if [ ! -e "$SAVEDIR/$OUT" ]; then
        timeout 40m dig axfr "@${HOST}" "${ZONE}" | gzip > "${SAVEDIR}/${OUT}.tmp"
        local status=$?
        if [ $status -ne 0 ]; then
            echo "Downloading $OUT failed"
            mv "$SAVEDIR/$OUT.tmp" "$SAVEDIR/$OUT.tmp.bad"
        else
            # count lines 
            lines=$(gzip -cd "$SAVEDIR/$OUT.tmp" | grep -v "^;" | grep -v -e '^$' | wc -l)
            if [ $lines -lt 50 ]
            then
                # no data
                rm "$SAVEDIR/$OUT.tmp"
            else
                mv "$SAVEDIR/$OUT.tmp" "$SAVEDIR/$OUT"
                echo "GOT $OUT"
            fi
        fi
    else
        echo "$OUT already exists"
    fi
}
export -f zoneTransfer


mkdir -p "$SAVEDIR"

curl -q "http://www.internic.net/domain/named.root" 2>/dev/null | grep -P "\sNS\s" | while read -r line
do
    #echo $line
    arrIN=(${line//\t/ })
    zone="${arrIN[0]}"
    ns="${arrIN[3]}"

    #echo "$zone --> $ns"

    runp zoneTransfer "$ns" "$zone" "${zone}_${ns}_zone.gz"
    #sleep 1
done

# wait for all tasks to finish before exiting script
sem --id "$$" --wait

