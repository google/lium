import Dygraph from 'dygraphs';

export class Graph {
  private g = new Dygraph('graph', [], {});
  public updateGraph(powerData: Array<Array<Date | number>>) {
    if (powerData !== undefined && powerData.length > 0) {
      const toolTip = document.querySelector('#tooltip');
      if (toolTip !== null) {
        toolTip.classList.add('hidden');
      }
    }
    // currentData = data;
    this.g.updateOptions(
      {
        file: powerData,
        labels: ['t', 'ina0'],
        showRoller: true,
        ylabel: 'Power (mW)',
        legend: 'always',
        showRangeSelector: true,
        connectSeparatedPoints: true,
        underlayCallback: function (canvas, area, g) {
          canvas.fillStyle = 'rgba(255, 255, 102, 1.0)';

          function highlight_period(x_start: number, x_end: number) {
            const canvas_left_x = g.toDomXCoord(x_start);
            const canvas_right_x = g.toDomXCoord(x_end);
            const canvas_width = canvas_right_x - canvas_left_x;
            canvas.fillRect(canvas_left_x, area.y, canvas_width, area.h);
          }
          highlight_period(10, 10);
        },
      },
      false
    );
  }
  public returnXrange() {
    console.log(this.g.xAxisExtremes());
    return this.g.xAxisRange();
  }
}
