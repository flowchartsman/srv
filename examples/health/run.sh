#!/usr/bin/env bash

tmpfile=$(mktemp)
go build -o ${tmpfile}
${tmpfile} --log-level debug &
pid=$!
if [[ "$OSTYPE" == "darwin"* ]]; then
	echo "you might need to click OK"
	sleep 5
fi
while true; do
	echo >&2 "Checking Health for ${pid}"
	statuscode=$(curl -sI "localhost:8081/livez" | head -n1 | cut -w -f 2)
	case ${statuscode} in
	"200")
		echo ${statuscode} - OK
		;;

	"500")
		echo ${statuscode} - FAIL
		break
		;;

	esac
	sleep 4
done
echo terminating program...
kill -s INT $pid
wait $pid
rm ${tmpfile}
