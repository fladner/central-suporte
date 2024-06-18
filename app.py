from flask import Flask, render_template, request, redirect, url_for, session
from flask_wtf import FlaskForm
from wtforms import StringField, PasswordField, SubmitField
from wtforms.validators import DataRequired
import subprocess

app = Flask(__name__)
app.secret_key = 'your_secret_key'

class LoginForm(FlaskForm):
    username = StringField('Username', validators=[DataRequired()])
    password = PasswordField('Password', validators=[DataRequired()])
    submit = SubmitField('Login')

def check_program_status():
    programs = ["inkscape", "netbeans", "blender", "brmodelo", "pycharm", "portugol", "workbench", "phpmyadmin", "code", "arduino", "nodejs"]
    statuses = {}
    for program in programs:
        result = subprocess.run(["dpkg", "-l", program], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        if result.returncode == 0:
            statuses[program] = 'installed'
        else:
            statuses[program] = 'not_installed'

        # Verificar se há atualizações disponíveis
        update_result = subprocess.run(["apt-get", "-s", "upgrade", program], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        if update_result.returncode == 0 and "Inst" in update_result.stdout:  # Verifica se a atualização está disponível
            statuses[program] = 'update_available'
    return statuses


@app.route('/', methods=['GET', 'POST'])
def login():
    form = LoginForm()
    if form.validate_on_submit():
        username = form.username.data
        password = form.password.data
        if username == 'admin' and password == 'admin_password':  # Replace with actual validation
            session['logged_in'] = True
            return redirect(url_for('install'))
        else:
            return render_template('login.html', form=form, error='Invalid Credentials')
    return render_template('login.html', form=form)

@app.route('/install', methods=['GET', 'POST'])
def install():
    if not session.get('logged_in'):
        return redirect(url_for('login'))
    if request.method == 'POST':
        selected_programs = request.form.getlist('programs')
        for program in selected_programs:
            subprocess.run(["sudo", "bash", "scripts/install_script.sh", program])
        return redirect(url_for('install'))
    statuses = check_program_status()
    return render_template('install.html', statuses=statuses)

@app.route('/uninstall', methods=['GET', 'POST'])
def uninstall():
    if not session.get('logged_in'):
        return redirect(url_for('login'))
    if request.method == 'POST':
        selected_programs = request.form.getlist('programs')
        for program in selected_programs:
            subprocess.run(["sudo", "apt-get", "remove", "-y", program])
        return redirect(url_for('uninstall'))
    statuses = check_program_status()
    return render_template('uninstall.html', statuses=statuses)

@app.route('/update', methods=['GET', 'POST'])
def update():
    if not session.get('logged_in'):
        return redirect(url_for('login'))

    if request.method == 'POST':
        selected_programs = request.form.getlist('programs')
        for program in selected_programs:
            subprocess.run(["sudo", "apt-get", "install", "--only-upgrade", "-y", program])
        return redirect(url_for('update'))

    statuses = check_program_status()
    update_available = {program: status for program, status in statuses.items() if status == 'update_available'}
    return render_template('update.html', statuses=update_available)

if __name__ == '__main__':
    app.run(debug=True)
