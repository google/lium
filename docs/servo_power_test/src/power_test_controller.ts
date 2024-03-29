import {ServoController} from './servo_controller';
import {Ui} from './ui';
import {DutController} from './dut_controller';
import {TestRunner} from './test_runner';
import {TotalHistogram} from './total_histogram';

export type PowerData = [number, number];
export type AnnotationDataList = Map<string, number>;

export class PowerTestController {
  private marginTime = 300;
  private ui: Ui;
  private servoController: ServoController;
  private dutController: DutController;
  private totalHistogram = new TotalHistogram();
  private testRunnerList: Array<TestRunner> = [];
  private currentRunnerNumber = 0;
  private iterationNumber = 2;
  public isMeasuring = false;
  constructor(
    ui: Ui,
    servoController: ServoController,
    dutController: DutController
  ) {
    this.ui = ui;
    this.servoController = servoController;
    this.dutController = dutController;
  }

  public setConfig() {
    const shellScriptContents = this.ui.readInputShellScript();
    this.ui.createGraphList();
    for (let i = 0; i < this.ui.runnerNumber; i++) {
      const newRunner = new TestRunner(
        this.ui,
        this.servoController,
        this.dutController,
        i,
        shellScriptContents[i],
        this.marginTime
      );
      this.testRunnerList.push(newRunner);
    }
  }
  private async initialize() {
    await this.servoController.initializePort();
    await this.dutController.initializePort();
  }
  private async finalize() {
    await this.dutController.dut.open();
    await this.dutController.sendCancelCommand();
    await this.dutController.sendCancelCommand();
    await this.dutController.sendCancelCommand();
    await this.dutController.dut.close();
  }
  public async startMeasurement() {
    if (this.ui.runnerNumber === 0) {
      this.ui.setErrorMessage(
        "No configuration/workload found.\nPlease click 'add config' button to setup a configuration/workload."
      );
      return;
    }
    this.marginTime = Number(this.ui.marginTimeInput.value);
    this.iterationNumber = parseInt(this.ui.iterationInput.value);
    if (this.iterationNumber <= 0) {
      this.ui.setErrorMessage(
        'Number of iterations <= 0.\nPlease set a number greater than or equal to 1.'
      );
      return;
    }
    await this.initialize().catch(e => {
      this.ui.setErrorMessage(e);
      throw e;
    });
    await this.setConfig();
    for (let i = 0; i < this.iterationNumber; i++) {
      this.ui.currentIteration.innerText = `${i + 1}`;
      for (let j = 0; j < this.ui.runnerNumber; j++) {
        this.currentRunnerNumber = j;
        await this.testRunnerList[j].start().catch(e => {
          this.ui.setErrorMessage(e + '\nPlease restart the measurement.');
          throw e;
        });
        if (this.testRunnerList[j].cancelled) return;
      }
    }
    this.finalize();
    this.drawTotalHistogram();
    this.ui.hideElement(this.ui.currentIteration);
    this.ui.appendIterationSelectors(
      this.iterationNumber,
      this.iterationNumber - 1
    );
  }
  public async cancelMeasurement() {
    await this.testRunnerList[this.currentRunnerNumber].cancel();
    await this.finalize();
  }
  private drawTotalHistogram() {
    const histogramData = [];
    for (const runner of this.testRunnerList) {
      const extractedData = runner.extractTotalHistogramData(this.marginTime);
      histogramData.push(extractedData);
    }
    this.totalHistogram.paintHistogram(histogramData);
  }
  public showSelectedIterationGraph(selectedIteration: number) {
    for (let i = 0; i < this.ui.runnerNumber; i++) {
      this.testRunnerList[i].loadGraph(selectedIteration);
    }
  }
  public loadPowerData(s: string) {
    const jsonData = JSON.parse(s);
    this.marginTime = jsonData.margin;
    this.ui.runnerNumber = jsonData.data.length;
    this.ui.createGraphList();
    this.testRunnerList = [];
    this.ui.appendIterationSelectors(this.iterationNumber, 0);
    for (let i = 0; i < jsonData.data.length; i++) {
      const runnerData = jsonData.data[i];
      const newRunner = new TestRunner(
        this.ui,
        this.servoController,
        this.dutController,
        i,
        runnerData.config,
        this.marginTime
      );
      runnerData.measuredData.map(
        (iterationData: {
          power: Array<{time: number; power: number}>;
          annotation: AnnotationDataList;
        }) => {
          const newPowerDataList = iterationData.power.map(
            (d: {time: number; power: number}) => [d.time, d.power] as PowerData
          );
          const newAnnotationList = new Map(
            Object.entries(iterationData.annotation)
          );
          newRunner.appendIterationDataList(
            newPowerDataList,
            newAnnotationList
          );
        }
      );
      newRunner.loadGraph(0);
      this.ui.loadConfigInputArea(runnerData.config);
      this.testRunnerList.push(newRunner);
    }
    this.drawTotalHistogram();
  }
  public exportPowerData() {
    const dataStr =
      'data:text/json;charset=utf-8,' +
      encodeURIComponent(
        JSON.stringify({
          margin: this.marginTime,
          iterationNumber: this.iterationNumber,
          data: this.testRunnerList.map(runner => ({
            config: runner.configScript,
            measuredData: runner.exportIterationDataList(),
          })),
        })
      );
    return dataStr;
  }
  public setupDisconnectEvent() {
    // event when you disconnect serial port
    navigator.serial.addEventListener('disconnect', async () => {
      this.cancelMeasurement();
    });
  }
}
