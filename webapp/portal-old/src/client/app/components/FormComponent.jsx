import React from 'react';
import KubernetesTypeWrapper from "../api/KubernetesTypeWrapper";

// @flow
class FormComponent extends React.Component {

  constructor(props, client: KubernetesTypeWrapper, name: string) {
    super(props);
    this.client = client;
    this.name = name;
    this.handleFormSubmit = this.handleFormSubmit.bind(this);
    this.handleInputChange = this.handleInputChange.bind(this);

    this.state = {data: null};
  }

  handleInputChange(event) {
    const target = event.target;
    const value = target.type === 'checkbox' ? target.checked : target.value;
    const name = target.name;

    var tokens = name.split(".");
    if (tokens.length == 1) {
      this.setState({
        [name]: value
      });
    } else {
      var newState = Object.assign({}, this.state);
      var current = newState;
      tokens.forEach(function (token, i) {
        if (i == tokens.length - 1) {
          current[token] = value;
        } else {
          var o = current[token];
          if (o == null || o == undefined) {
            o = {};
            current[token] = o;
          }
          current = o;
        }
      });
      this.setState(newState);
    }
  }

  componentDidMount() {
    this.client.get(this.name)
      .then(json => {
        this.setState({
          data: json,
        });
      });
  }

  handleFormSubmit(e) {
    e.preventDefault();

    const formPayload = this.state.data;

    console.log('PUT :', formPayload);

    this.client.put(this.name, formPayload);
  }

  errorText(path) {
    return "";
  }
}

export default FormComponent;