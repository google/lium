// while true ; do { echo "do nothing for 5 sec" ; sleep 5 ; echo "yes for 5 sec
// without displaying" ; timeout 5 yes > /dev/null ; } ; done ectool
// chargecontrol idle ectool chargecontrol normal

import * as d3 from 'd3';
import Dygraph from 'dygraphs';
import moment from 'moment';

const intervalMs = 100;

let downloadButton =
    document.getElementById('downloadButton') as HTMLButtonElement;
let requestUSBButton =
    document.getElementById('request-device') as HTMLButtonElement;
const requestSerialButton =
    document.getElementById('requestSerialButton') as HTMLButtonElement;
const serial_output =
    document.getElementById('serial_output') as HTMLDivElement;
const controlDiv = document.getElementById('controlDiv') as HTMLDivElement;
const selectButton = document.getElementById('select');

const encoder = new TextEncoder();
const decoder = new TextDecoder();
let DUTPort;

async function DUTLogin(DUTWriter: WritableStreamDefaultWriter, listItemText: string) {
  const loginMessage = "localhost login:"
  console.log(listItemText);
  if (listItemText.includes(loginMessage)) {
    await DUTWriter.write("root\n");
    await DUTWriter.write("test0000\n");
    await DUTWriter.releaseLock();
  }
}

function addListItem(listItem) {
  const messages = document.getElementById('messages');
  messages.appendChild(listItem);
  messages.scrollTo(0, messages.scrollHeight);
}

let powerData = [];
const g = new Dygraph('graph', powerData, {});
const utf8decoder = new TextDecoder('utf-8');
let output = '';
let halt = false;

let currentData = undefined;
function updateGraph(data: Array<Array<Date|number>>) {
  if (data !== undefined && data.length > 0) {
    document.querySelector('#tooltip').classList.add("hidden");
  }
  currentData = data;
  g.updateOptions({
    file : data,
    labels : [ 't', 'ina0' ],
    showRoller : true,
    ylabel : 'Power (mW)',
    legend : 'always',
    showRangeSelector : true,
    connectSeparatedPoints : true,
    underlayCallback : function(canvas, area, g) {
      canvas.fillStyle = 'rgba(255, 255, 102, 1.0)';

      function highlight_period(x_start: number, x_end: number) {
        const canvas_left_x = g.toDomXCoord(x_start);
        const canvas_right_x = g.toDomXCoord(x_end);
        const canvas_width = canvas_right_x - canvas_left_x;
        canvas.fillRect(canvas_left_x, area.y, canvas_width, area.h);
      }
      highlight_period(10, 10);
    }
  },
                  false);
}

let inProgress = false;
function pushOutput(s: string) {
  output += s

  let splitted = output.split('\n').filter((s) => s.trim().length > 10);
  if (splitted.length > 0 &&
      splitted[splitted.length - 1].indexOf('Alert limit') >= 0) {
    let power = parseInt(splitted.find((s) => s.startsWith('Power'))
                             .split('=>')[1]
                             .trim()
                             .split(' ')[0]);
    let e: Array<Date|Number> = [ new Date(), power ];
    powerData.push(e);
    updateGraph(powerData);
    serial_output.innerText = output;
    output = '';
    inProgress = false;
  }
}

function kickWriteLoop(writeFn: (s: string) => Promise<void>) {
  const f = async (_: any) => {
    while (!halt) {
      if (inProgress) {
        console.error('previous request is in progress! skip...');
      } else {
        inProgress = true;
      }

      // ina 0 and 1 seems to be the same
      // ina 2 is something but not useful
      const cmd = `ina 0\n`;
      await writeFn(cmd);
      await new Promise(r => setTimeout(r, intervalMs));
    }
  };
  setTimeout(f, intervalMs);
}
async function readLoop(readFn: () => Promise<string>) {
  while (!halt) {
    try {
      const s = await readFn();
      if (s === undefined || !s.length) {
        continue;
      }
      pushOutput(s);
    } catch (e) {
      // break the loop here because `disconnect` event is not called in Chrome
      // for some reason when the loop continues. And no need to throw error
      // here because it is thrown in readFn.
      break;
    }
  }
}

let device: USBDevice;
function closeUSBPort() {
  try {
    device.close();
  } catch (e) {
    console.error(e);
  }
  requestUSBButton.disabled = false;
}

let servoPort;
let reader: ReadableStreamDefaultReader;
function closeSerialPort() {
  reader.cancel();
  reader.releaseLock();
  try {
    servoPort.close();
  } catch (e) {
    console.error(e);
  }
  requestSerialButton.disabled = false;
}

function setupStartUSBButton() {
  let usb_interface = 0;
  let ep = usb_interface + 1;
  requestUSBButton.addEventListener('click', async () => {
    halt = false;
    device = null;
    try {
      device = await navigator.usb.requestDevice({
        filters : [ {
          vendorId : 0x18d1,  /* Google */
          productId : 0x520d, /* Servo v4p1 */
        } ]
      });
    } catch (err) {
      console.error(`Error: ${err}`);
    }
    if (!device) {
      device = null;
      return;
    }

    try {
      await device.open();
      requestUSBButton.disabled = true;
      await device.selectConfiguration(1);
      await device.claimInterface(usb_interface);
      kickWriteLoop(async (s) => {
        let data = new TextEncoder().encode(s);
        await device.transferOut(ep, data);
      })
      readLoop(async () => {
        try {
          let result = await device.transferIn(ep, 64);
          if (result.status === 'stall') {
            await device.clearHalt('in', ep);
            throw result;
          }
          const result_array = new Int8Array(result.data.buffer);
          return utf8decoder.decode(result_array);
        } catch (e) {
          // If halt is true, it's when the stop button is pressed. Therefore,
          // we can ignore the error.
          if (!halt) {
            console.error(e);
            throw e;
          }
        }
      });
    } catch (err) {
      console.error(`Disconnected: ${err}`);
      device = null;
      requestUSBButton.disabled = false;
    }
  });
  window.addEventListener('keydown', async (event) => {
    if (!device) {
      return;
    }
    let data: any;
    if (event.key.length === 1) {
      data = new Int8Array([ event.key.charCodeAt(0) ]);
    } else if (event.code === 'Enter') {
      data = new Uint8Array([ 0x0a ]);
    } else {
      return;
    }
    await device.transferOut(ep, data);
  }, true);
};
setupStartUSBButton();

requestSerialButton.addEventListener('click', async () => {
  halt = false;

  DUTPort =
      await navigator.serial.requestPort().catch((e) => { console.error(e); });
  await DUTPort.open({baudRate : 115200});
  let listItem = document.createElement("li");
  listItem.textContent = "DUTPort is selected";
  addListItem(listItem);

  const DUTReader = DUTPort.readable.getReader();
  const DUTWriter = DUTPort.writable.getWriter();
  listItem = document.createElement("li");
  addListItem(listItem);
  DUTReader.read().then(function processText({done, value}) {
    if (done) {
      console.log("Stream complete");
      return;
    }
    
    const chunk = decoder.decode(value, {stream : true});
    const chunk_split_list = chunk.split("\n");
    
    for (let i = 0; i < chunk_split_list.length - 1; i++) {
      listItem.textContent += chunk_split_list[i];
      listItem = document.createElement("li");
      addListItem(listItem);
    }
    listItem.textContent += chunk_split_list[chunk_split_list.length - 1];

    return DUTReader.read().then(processText);
  });
  servoPort =
  await navigator.serial
  .requestPort(
    {filters : [ {usbVendorId : 0x18d1, usbProductId : 0x520d} ]})
    .catch((e) => { console.error(e); });
    await servoPort.open({baudRate : 115200});
    requestSerialButton.disabled = true;
    const encoder = new TextEncoder();
    const writer = servoPort.writable.getWriter();
    await writer.write(encoder.encode('help\n'));
    writer.releaseLock();
    
    const scripts = `#!/bin/bash -e
    function bench_run_cpu \{
      CPU=\$1
      TOTAL_SIZE=\$(echo "4 * 1024 * 1024" | bc)
      BLOCK_SIZE=2
      COUNT=\$(echo "\$\{TOTAL_SIZE\} / \$\{BLOCK_SIZE\}" | bc)
      CMD="dd if=/dev/urandom bs=\$\{BLOCK_SIZE\} count=\$\{COUNT\} | wc -c"
      RESULT=\$(taskset -c \$\{CPU\} /bin/bash -c "\$\{CMD\}" 2>&1 | grep copied)
      REALTIME=\$(echo "\$\{RESULT\}" | cut -d ',' -f 3 | cut -d ' ' -f 2)
      echo "\$\{CPU\},\$\{TOTAL_SIZE\},\$\{BLOCK_SIZE\},\$\{COUNT\},\$\{REALTIME\}"
    \}
    for ((CPU_IDX=0; CPU_IDX<\$(nproc); CPU_IDX++))
    do
      bench_run_cpu \$\{CPU_IDX\}
    done\n`
    // await DUTWriter.write(encoder.encode("\n"));
    await DUTWriter.write(encoder.encode("cat > ./example.sh << EOF\n"));
    await DUTWriter.write(encoder.encode(scripts));
    await DUTWriter.write(encoder.encode("EOF\n"));
    // await DUTWriter.write(encoder.encode("cat ./example.sh\n"));
    DUTWriter.releaseLock();

  kickWriteLoop(async (s) => {
    let data = new TextEncoder().encode(s);
    const writer = servoPort.writable.getWriter();
    await writer.write(data);
    writer.releaseLock();
  })
  readLoop(async () => {
    reader = servoPort.readable.getReader();
    try {
      while (true) {
        const {value, done} = await reader.read();
        if (done) {
          // |reader| has been canceled.
          break;
        }
        return utf8decoder.decode(value);
      }
    } catch (error) {
      console.error(error);
      throw error;
    } finally {
      reader.releaseLock();
    }
  });
});

// `disconnect` event is fired when a USB device is disconnected.
// c.f. https://wicg.github.io/webusb/#disconnect (5.1. Events)
navigator.usb.addEventListener("disconnect", () => {
  if (requestUSBButton.disabled) {
    //  No need to call close() for the USB servoPort here because the
    //  specification says that
    // the servoPort will be closed automatically when a device is disconnected.
    halt = true;
    requestUSBButton.disabled = false;
    inProgress = false;
  }
});

// event when you disconnect serial servoPort
navigator.serial.addEventListener("disconnect", () => {
  if (requestSerialButton.disabled) {
    halt = true;
    inProgress = false;
    closeSerialPort();
  }
});

downloadButton.addEventListener('click', async () => {
  const dataStr = 'data:text/json;charset=utf-8,' +
                  encodeURIComponent(JSON.stringify({power : powerData}));
  const dlAnchorElem = document.getElementById('downloadAnchorElem');
  dlAnchorElem.setAttribute('href', dataStr);
  dlAnchorElem.setAttribute('download', `power_${moment().format()}.json`);
  dlAnchorElem.click();
});

let haltButton = document.getElementById('haltButton') as HTMLButtonElement;
haltButton.addEventListener('click', () => {
  halt = true;
  if (requestUSBButton.disabled) {
    closeUSBPort();
  }
  if (requestSerialButton.disabled) {
    closeSerialPort();
  }
});

let ranges = [];
function paintHistogram(t0: number, t1: number) {
  // constants
  const xtick = 40;
  const boxWidth = 10;

  // setup a graph (drop if exists)
  const margin = {top : 60, right : 200, bottom : 0, left : 200};
  const area = d3.select('#d3area');
  var targetWidth =
      (area.node() as HTMLElement).getBoundingClientRect().width * 0.98;
  var targetHeight =
      10000; // (area.node() as HTMLElement).getBoundingClientRect().height;
  const width = targetWidth - margin.left - margin.right;
  const height = targetHeight - margin.top - margin.bottom;
  const svg = area.html('')
                  .append('svg')
                  .attr('height', targetHeight)
                  .attr('width', targetWidth)
                  .append('g')
                  .attr('transform',
                        'translate(' + margin.left + ',' + margin.top + ')');

  // y axis and its label
  const dataAll: Array<number> =
      currentData.map((e: (Date|number)) => e[1] as number);
  const ymin = d3.min(dataAll) - 1000;
  const ymax = d3.max(dataAll) + 1000;
  const y = d3.scaleLinear().domain([ ymin, ymax ]).range([ 0, width ]);
  svg.append('g').call(d3.axisTop(y));
  svg.append('text')
      .attr('text-anchor', 'end')
      .attr('x', width)
      .attr('y', -margin.top / 2)
      .attr('stroke', '#fff')
      .text('Power (mW)');

  ranges.push([ t0, t1 ]);

  for (let i = 0; i < ranges.length; i++) {
    // compute data and place of i-th series
    const left = ranges[i][0];
    const right = ranges[i][1];
    let points =
        currentData.filter((e: (Date|String)) => (left <= e[0].getTime() &&
                                                  e[0].getTime() <= right));
    let data: Array<number> = points.map((e: (Date|number)) => e[1] as number);
    const center = xtick * (i + 1);

    // Compute statistics
    const data_sorted = data.sort(d3.ascending);
    const q1 = d3.quantile(data_sorted, .25);
    const median = d3.quantile(data_sorted, .5);
    const q3 = d3.quantile(data_sorted, .75);
    const interQuantileRange = q3 - q1;
    const lowerFence = q1 - 1.5 * interQuantileRange;
    const upperFence = q3 + 1.5 * interQuantileRange;
    const minValue = d3.min(data);
    const maxValue = d3.max(data);
    const mean = d3.mean(data);

    // min, mean, max
    svg.append('line')
        .attr('y1', center)
        .attr('y2', center)
        .attr('x1', y(minValue))
        .attr('x2', y(maxValue))
        .style('stroke-dasharray', '3, 3')
        .attr('stroke', '#aaa')
    svg.selectAll('toto')
        .data([ minValue, mean, maxValue ])
        .enter()
        .append('line')
        .attr('y1', center - boxWidth)
        .attr('y2', center + boxWidth)
        .attr('x1', function(d) { return (y(d)) })
        .attr('x2', function(d) { return (y(d)) })
        .style('stroke-dasharray', '3, 3')
        .attr('stroke', '#aaa');

    // box and line
    svg.append('line')
        .attr('y1', center)
        .attr('y2', center)
        .attr('x1', y(lowerFence))
        .attr('x2', y(upperFence))
        .attr('stroke', '#fff')
    svg.append('rect')
        .attr('y', center - boxWidth / 2)
        .attr('x', y(q1))
        .attr('width', (y(q3) - y(q1)))
        .attr('height', boxWidth)
        .attr('stroke', '#fff')
        .style('fill', '#69b3a2')
    svg.selectAll('toto')
        .data([ lowerFence, median, upperFence ])
        .enter()
        .append('line')
        .attr('y1', center - boxWidth / 2)
        .attr('y2', center + boxWidth / 2)
        .attr('x1', function(d) { return (y(d)) })
        .attr('x2', function(d) { return (y(d)) })
        .attr('stroke', '#fff');

    svg.append('text')
        .attr('text-anchor', 'end')
        .attr('alignment-baseline', 'baseline')
        .attr('y', center - boxWidth / 4)
        .attr('x', 0)
        .attr('font-size', boxWidth)
        .attr('stroke', '#fff')
        .text(`${moment(left).format()}`);
    svg.append('text')
        .attr('text-anchor', 'end')
        .attr('alignment-baseline', 'hanging')
        .attr('y', center + boxWidth / 4)
        .attr('x', 0)
        .attr('font-size', boxWidth)
        .attr('stroke', '#fff')
        .text(`${moment(right).format()}`);

    svg.append('text')
        .attr('text-anchor', 'middle')
        .attr('alignment-baseline', 'baseline')
        .attr('y', center - boxWidth)
        .attr('x', y(mean))
        .attr('font-size', boxWidth)
        .attr('stroke', '#fff')
        .text(`mean:${mean | 0}`);

    svg.append('text')
        .attr('text-anchor', 'middle')
        .attr('alignment-baseline', 'hanging')
        .attr('y', center + boxWidth)
        .attr('x', y(median))
        .attr('font-size', boxWidth)
        .attr('stroke', '#fff')
        .text(`median:${median}`);

    svg.append('text')
        .attr('text-anchor', 'start')
        .attr('alignment-baseline', 'hanging')
        .attr('y', center + boxWidth)
        .attr('x', y(ymax))
        .attr('font-size', boxWidth)
        .attr('stroke', '#fff')
        .text(`N:${data.length}`);
  }
}

function setupAnalyze() {
  const button = document.createElement('button');
  button.innerText = 'Analyze displayed range';
  controlDiv.appendChild(button);
  button.addEventListener('click', () => {
    // https://dygraphs.com/jsdoc/symbols/Dygraph.html#xAxisRange
    let xrange = g.xAxisRange();
    console.log(g.xAxisExtremes())
    let left = xrange[0];
    let right = xrange[1];
    paintHistogram(left, right);
  });
}
setupAnalyze();

function setupDataLoad() {
  const handleFileSelect = (evt: DragEvent) => {
    evt.stopPropagation();
    evt.preventDefault();
    const file = evt.dataTransfer.files[0];
    if (file === undefined) {
      return;
    }
    const r = new FileReader();
    r.addEventListener('load', () => {
      const data = JSON.parse(r.result as string);
      const powerData = data.power.map((d: string) => [new Date(d[0]), d[1]])
      updateGraph(powerData);
    })
    r.readAsText(file);
  };

  const handleDragOver = (evt: DragEvent) => {
    evt.stopPropagation();
    evt.preventDefault();
    evt.dataTransfer.dropEffect = 'copy'; // Explicitly show this is a copy.
  };
  const dropZone = document.getElementById('dropZone');
  dropZone.innerText = 'Drop .json here';
  dropZone.addEventListener('dragover', handleDragOver, false);
  dropZone.addEventListener('drop', handleFileSelect, false);
}
setupDataLoad();
