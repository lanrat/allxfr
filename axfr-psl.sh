#! /usr/bin/env bash

DATE=$(date +%Y-%m-%d)
SAVEDIR="zones/${DATE}/"

export DATE
export SAVEDIR

export JOBS=10
export IP_JOBS=5
export MINLINES=5

# run in parallel using sem
function runpAXFR {
    sem --ungroup --id "axfr$$" --jobs "$JOBS" "$@"
}
export -f runpAXFR

function runpIP {
    sem --ungroup --id "ip$$" --jobs "$IP_JOBS" "$@"
}


function zoneTransfer {
    HOST=$1
    ZONE=$2
    OUT=$3
    #echo "AXFR $ZONE $HOST"
    if [ ! -e "$SAVEDIR/$OUT" ]; then
        timeout 60m dig +answer +noidnout -t axfr "@${HOST}" -q "${ZONE}" | gzip > "${SAVEDIR}/${OUT}.tmp"
        local status=$?
        if [ $status -ne 0 ]; then
            echo "Downloading $OUT failed"
            mv "$SAVEDIR/$OUT.tmp" "$SAVEDIR/$OUT.tmp.bad"
        else
            # count lines 
            # TODO add head -$MINLINES here?
            lines=$(gzip -cd "$SAVEDIR/$OUT.tmp" | grep -v "^;" | grep -v -e '^$' | wc -l)
            #echo "$ZONE have $lines "
            if [ $lines -lt $MINLINES ]
            then
                # no data
                rm "$SAVEDIR/$OUT.tmp"
            else
                mv "$SAVEDIR/$OUT.tmp" "$SAVEDIR/$OUT"
                echo "FOUND $ZONE $OUT"
            fi
        fi
    else
        echo "$OUT already exists"
    fi
}
export -f zoneTransfer


function getIPs {
    zone="${1,,}"
    nameservers="$(dig +noidnout +short -t NS -q $zone)"

    for ns in ${nameservers,,}
    do
        #echo "Trying $zone $ns"
        ips="$(dig +noidnout +short -t A -q $ns ; dig +noidnout +short -t AAAA -q $ns)"
        for ip in ${ips}
        do
            #echo "$zone $ns: $ip"
            out_file="${zone}._${ns}_${ip}_zone.gz"
            runpAXFR zoneTransfer "$ip" "$zone" "$out_file"
        done
    done
    
}
export -f getIPs

mkdir -p "$SAVEDIR"

curl -q "https://publicsuffix.org/list/public_suffix_list.dat" | sed -n '/===BEGIN ICANN DOMAINS===/,/===END ICANN DOMAINS===/p;/===END ICANN DOMAINS===/q' | grep -v "//" | grep -v "^$" | sed 's/*.//g' | grep -v "^!" | idn | while read -r line
do
    zone="$line"

    # skip arpa
    if [[ "${zone^^}" == "arpa" ]]
    then
        continue
    fi
    if [[ "${zone^^}" == "arpa."* ]]
    then
        continue
    fi
 
    #echo "Zone: $zone"
    runpIP getIPs "$zone"
done

# wait for all tasks to finish before exiting script
sem --id "ip$$" --wait
sleep 1
sem --id "axfr$$" --wait

echo "found $(ls ${SAVEDIR}/*.gz | wc -l) zones"
